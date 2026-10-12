package items

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Loans own the `lent` status (MILESTONES M9): lending sets it, returning
// restores the status the item had before, and the generic status paths
// refuse to set or clear it so item and loan cannot disagree.
const (
	lendInsteadMsg   = "use lend to mark an item as lent"
	returnInsteadMsg = "this item is lent out; mark it returned instead"
	maxBorrowerName  = 200
)

// checkLoanOwnedStatus rejects a generic status change that would set or
// clear `lent`. A lent item may still reach a terminal status; the loan is
// closed alongside (closeLoanIfLent).
func checkLoanOwnedStatus(from, to api.ItemStatus) error {
	switch {
	case to == api.Lent:
		return &ValidationError{Msg: lendInsteadMsg}
	case from == api.Lent && IsActive(to):
		return &ValidationError{Msg: returnInsteadMsg}
	}

	return nil
}

// closeLoanIfLent closes the open loan of an item that has just left the
// `lent` status for a terminal one. No-op for any other transition.
func closeLoanIfLent(ctx context.Context, q *db.Queries, familyID, itemID pgtype.UUID, from api.ItemStatus) error {
	if from != api.Lent {
		return nil
	}

	loan, err := q.GetOpenLoanForItem(ctx, db.GetOpenLoanForItemParams{ItemID: itemID, FamilyID: familyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("get open loan: %w", err)
	}

	if _, err := q.ReturnLoan(ctx, db.ReturnLoanParams{ID: loan.ID, FamilyID: familyID}); err != nil {
		return fmt.Errorf("close loan: %w", err)
	}

	return nil
}

// Lend opens a loan on an item and moves it to `lent`.
func (s *Service) Lend(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.LendItemRequest,
) (api.Loan, error) {
	borrowerName, err := validateLend(body)
	if err != nil {
		return api.Loan{}, err
	}

	familyID := pgconv.UUID(principal.FamilyID)

	var loan db.Loan

	err = s.tx.InTx(ctx, func(q *db.Queries) error {
		item, err := q.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		from := api.ItemStatus(item.Status)
		if item.ArchivedAt.Valid || !IsActive(from) {
			return &ValidationError{Msg: "only an item in use or stored can be lent"}
		}

		if from == api.Lent {
			return &ValidationError{Msg: "this item is already lent out"}
		}

		loan, err = q.CreateLoan(ctx, db.CreateLoanParams{
			FamilyID: familyID, ItemID: item.ID,
			BorrowerPersonID: pgconv.UUIDPtr(body.BorrowerPersonId),
			BorrowerName:     pgconv.TextPtr(borrowerName),
			DueAt:            pgconv.Date(dateToTimePtr(body.DueAt)),
			Note:             pgconv.TextPtr(trimmedPtr(body.Note)),
			RestoreStatus:    item.Status,
			CreatedBy:        pgconv.UUID(principal.UserID),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return &ValidationError{Msg: "no such person in this family"}
		}

		if err != nil {
			return fmt.Errorf("create loan: %w", err)
		}

		if _, err := q.SetItemStatus(ctx, db.SetItemStatusParams{
			Status: db.ItemStatus(api.Lent), ID: item.ID, FamilyID: familyID,
		}); err != nil {
			return fmt.Errorf("set item status: %w", err)
		}

		return s.writeLoanEvent(ctx, q, principal, &item, eventLent, lentPayload(&loan, borrowerName, from))
	})
	if err != nil {
		if isUniqueViolation(err) {
			return api.Loan{}, &ValidationError{Msg: "this item is already lent out"}
		}

		return api.Loan{}, err
	}

	return s.loanToAPI(ctx, familyID, &loan)
}

// Return closes an item's open loan and restores its previous status.
func (s *Service) Return(ctx context.Context, principal auth.Principal, itemID uuid.UUID) (api.Loan, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	var closed db.Loan

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		item, err := q.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		open, err := q.GetOpenLoanForItem(ctx, db.GetOpenLoanForItemParams{ItemID: item.ID, FamilyID: familyID})
		if errors.Is(err, pgx.ErrNoRows) {
			return &ValidationError{Msg: "this item is not lent out"}
		}

		if err != nil {
			return fmt.Errorf("get open loan: %w", err)
		}

		closed, err = q.ReturnLoan(ctx, db.ReturnLoanParams{ID: open.ID, FamilyID: familyID})
		if err != nil {
			return fmt.Errorf("return loan: %w", err)
		}

		restore := api.ItemStatus(open.RestoreStatus)
		if _, err := q.SetItemStatus(ctx, db.SetItemStatusParams{
			Status: db.ItemStatus(restore), ID: item.ID, FamilyID: familyID,
		}); err != nil {
			return fmt.Errorf("restore item status: %w", err)
		}

		return s.writeLoanEvent(ctx, q, principal, &item, eventReturned, map[string]any{
			"loanId": pgconv.ToUUID(open.ID).String(), "status": string(restore),
		})
	})
	if err != nil {
		return api.Loan{}, err
	}

	return s.loanToAPI(ctx, familyID, &closed)
}

// ListItemLoans returns an item's loan history, newest first.
func (s *Service) ListItemLoans(ctx context.Context, principal auth.Principal, itemID uuid.UUID) ([]api.Loan, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	rows, err := s.store.ListItemLoans(ctx, db.ListItemLoansParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list item loans: %w", err)
	}

	today := utcDay(s.now())
	out := make([]api.Loan, 0, len(rows))

	for i := range rows {
		r := &rows[i]
		loan := loanFields(r.ID, r.ItemID, r.BorrowerPersonID, r.BorrowerName, r.PersonName, r.LentAt, r.DueAt, r.ReturnedAt, r.Note, today)
		out = append(out, loan)
	}

	return out, nil
}

// ListOpenLoans returns everything currently lent out, overdue first.
func (s *Service) ListOpenLoans(ctx context.Context, principal auth.Principal) ([]api.Loan, error) {
	rows, err := s.store.ListOpenLoans(ctx, pgconv.UUID(principal.FamilyID))
	if err != nil {
		return nil, fmt.Errorf("list open loans: %w", err)
	}

	today := utcDay(s.now())
	out := make([]api.Loan, 0, len(rows))

	for i := range rows {
		r := &rows[i]
		loan := loanFields(r.ID, r.ItemID, r.BorrowerPersonID, r.BorrowerName, r.PersonName, r.LentAt, r.DueAt, r.ReturnedAt, r.Note, today)
		loan.ItemName = &r.ItemName
		categoryID := pgconv.ToUUID(r.CategoryID)
		loan.CategoryId = &categoryID
		out = append(out, loan)
	}

	// The query orders by due date; overdue loans are the ones whose date has
	// passed, which sort first among dated loans already.
	return out, nil
}

// loanToAPI renders a freshly written loan, looking up the borrower's name.
func (s *Service) loanToAPI(ctx context.Context, familyID pgtype.UUID, l *db.Loan) (api.Loan, error) {
	rows, err := s.store.ListItemLoans(ctx, db.ListItemLoansParams{ItemID: l.ItemID, FamilyID: familyID})
	if err != nil {
		return api.Loan{}, fmt.Errorf("load loan: %w", err)
	}

	today := utcDay(s.now())

	for i := range rows {
		if rows[i].ID == l.ID {
			r := &rows[i]

			return loanFields(r.ID, r.ItemID, r.BorrowerPersonID, r.BorrowerName, r.PersonName, r.LentAt, r.DueAt, r.ReturnedAt, r.Note, today), nil
		}
	}

	return api.Loan{}, &NotFoundError{What: "loan"}
}

func loanFields(
	id, itemID, personID pgtype.UUID, name, personName pgtype.Text,
	lentAt pgtype.Timestamptz, dueAt pgtype.Date, returnedAt pgtype.Timestamptz, note pgtype.Text, today time.Time,
) api.Loan {
	loan := api.Loan{
		Id: pgconv.ToUUID(id), ItemId: pgconv.ToUUID(itemID),
		BorrowerPersonId: pgconv.ToUUIDPtr(personID),
		BorrowerName:     pgconv.ToTextPtr(personName),
		LentAt:           pgconv.ToTime(lentAt),
		ReturnedAt:       pgconv.ToTimePtr(returnedAt),
		Note:             pgconv.ToTextPtr(note),
	}

	if loan.BorrowerName == nil {
		loan.BorrowerName = pgconv.ToTextPtr(name)
	}

	if due := pgconv.ToDatePtr(dueAt); due != nil {
		loan.DueAt = &openapi_types.Date{Time: *due}
		loan.Overdue = !returnedAt.Valid && due.Before(today)
	}

	return loan
}

func validateLend(body api.LendItemRequest) (*string, error) {
	hasPerson := body.BorrowerPersonId != nil
	name := trimmedPtr(body.BorrowerName)

	if hasPerson == (name != nil) {
		return nil, &ValidationError{Msg: "give either a person or a name to lend to"}
	}

	if name != nil && len([]rune(*name)) > maxBorrowerName {
		return nil, &ValidationError{Msg: "the borrower's name is too long"}
	}

	return name, nil
}

// trimmedPtr trims a string pointer and treats blank as absent.
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}

	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}

	return &t
}

func lentPayload(loan *db.Loan, borrowerName *string, from api.ItemStatus) map[string]any {
	payload := map[string]any{
		"loanId": pgconv.ToUUID(loan.ID).String(),
		"from":   string(from),
	}

	if p := pgconv.ToUUIDPtr(loan.BorrowerPersonID); p != nil {
		payload["borrowerPersonId"] = p.String()
	}

	if borrowerName != nil {
		payload["borrowerName"] = *borrowerName
	}

	if due := pgconv.ToDatePtr(loan.DueAt); due != nil {
		payload["dueAt"] = due.Format(time.DateOnly)
	}

	return payload
}

func (s *Service) writeLoanEvent(
	ctx context.Context, q *db.Queries, principal auth.Principal, item *db.Item, kind string, payload map[string]any,
) error {
	if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
		FamilyID: item.FamilyID, ItemID: item.ID, ActorUserID: pgconv.UUID(principal.UserID),
		Type: kind, Payload: pgconv.JSONB(payload),
	}); err != nil {
		return fmt.Errorf("create item event: %w", err)
	}

	return nil
}
