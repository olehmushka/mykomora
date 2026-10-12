package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Limits on the fast-entry paths, so one request cannot hold a transaction
// open for an unreasonable time.
const (
	maxBatchItems     = 100
	maxDuplicateCount = 50
)

// CreateItems adds every item in rows or none of them. Each row is a
// complete create request — the web client merges the shared fields into
// each row — so a row is validated exactly as a single create would be,
// and a failure names the row.
func (s *Service) CreateItems(ctx context.Context, principal auth.Principal, rows []api.CreateItemRequest) ([]api.Item, error) {
	if len(rows) == 0 {
		return nil, &ValidationError{Msg: "a batch needs at least one item"}
	}

	if len(rows) > maxBatchItems {
		return nil, &ValidationError{Msg: fmt.Sprintf("a batch holds at most %d items", maxBatchItems)}
	}

	prepared := make([]newItem, 0, len(rows))

	for i := range rows {
		params, tagIDs, err := s.prepareCreate(ctx, principal, &rows[i])
		if err != nil {
			return nil, rowError(i, err)
		}

		prepared = append(prepared, newItem{params: params, tagIDs: tagIDs})
	}

	return s.insertMany(ctx, principal, prepared)
}

// DuplicateItem makes count copies of an item in the same place. Photos,
// attachments, events and contents are not copied; the copy is a fresh
// item of the same kind, so its status starts active.
func (s *Service) DuplicateItem(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, count int,
) ([]api.Item, error) {
	if count < 1 || count > maxDuplicateCount {
		return nil, &ValidationError{Msg: fmt.Sprintf("count must be between 1 and %d", maxDuplicateCount)}
	}

	familyID := pgconv.UUID(principal.FamilyID)

	src, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	tags, err := s.store.ListTagsForItem(ctx, db.ListTagsForItemParams{ItemID: src.ID, FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list tags for item: %w", err)
	}

	tagIDs := make([]uuid.UUID, 0, len(tags))
	for _, t := range tags {
		tagIDs = append(tagIDs, pgconv.ToUUID(t.ID))
	}

	prepared := make([]newItem, 0, count)

	for range count {
		prepared = append(prepared, newItem{
			params: duplicateParams(&src, principal),
			tagIDs: tagIDs,
			extra:  map[string]any{"duplicatedFrom": itemID.String()},
		})
	}

	return s.insertMany(ctx, principal, prepared)
}

// duplicateParams copies every user-facing field of src. A copy of a
// terminal-status item (lent, sold...) starts as in_use again.
func duplicateParams(src *db.Item, principal auth.Principal) *db.CreateItemParams {
	status := src.Status
	if !IsActive(api.ItemStatus(status)) {
		status = db.ItemStatus(api.InUse)
	}

	return &db.CreateItemParams{
		FamilyID: src.FamilyID, Name: src.Name, CategoryID: src.CategoryID,
		LocationID: src.LocationID, RoomID: src.RoomID, Spot: src.Spot, ParentItemID: src.ParentItemID,
		IsContainer: false, Status: status,
		Quantity: src.Quantity, Unit: src.Unit, MinQuantity: src.MinQuantity,
		ExpiresAt: src.ExpiresAt, OpenedAt: src.OpenedAt, PaoMonths: src.PaoMonths,
		Brand: src.Brand, Condition: src.Condition, Season: src.Season,
		SizeSystem: src.SizeSystem, SizeValue: src.SizeValue, ForPersonID: src.ForPersonID,
		SerialNumber: pgtype.Text{}, WarrantyUntil: src.WarrantyUntil,
		PurchaseDate: src.PurchaseDate, PurchasePrice: src.PurchasePrice, PurchaseCurrency: src.PurchaseCurrency,
		Notes: src.Notes, Traits: src.Traits,
		CreatedBy: pgconv.UUID(principal.UserID),
	}
}

// insertRows writes every prepared item in one transaction.
func (s *Service) insertRows(ctx context.Context, principal auth.Principal, prepared []newItem) ([]db.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)
	created := make([]db.Item, 0, len(prepared))

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		created = created[:0]

		for _, n := range prepared {
			row, err := insertItem(ctx, q, principal, familyID, n)
			if err != nil {
				return err
			}

			created = append(created, row)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

// insertMany is insertRows plus the API representation of each new item, in
// order.
func (s *Service) insertMany(ctx context.Context, principal auth.Principal, prepared []newItem) ([]api.Item, error) {
	created, err := s.insertRows(ctx, principal, prepared)
	if err != nil {
		return nil, err
	}

	out := make([]api.Item, 0, len(created))

	for i := range created {
		item, err := s.toAPIItem(ctx, principal.FamilyID, &created[i])
		if err != nil {
			return nil, err
		}

		out = append(out, item)
	}

	return out, nil
}

// rowError prefixes a validation or not-found failure with the 1-based row
// it came from, keeping its type so the handler maps it to the same status.
func rowError(i int, err error) error {
	if msg := asValidation(err); msg != "" {
		return &ValidationError{Msg: fmt.Sprintf("row %d: %s", i+1, msg)}
	}

	if what := asNotFound(err); what != "" {
		return &ValidationError{Msg: fmt.Sprintf("row %d: %s not found", i+1, what)}
	}

	return err
}
