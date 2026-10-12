package items

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// How far ahead the Attention panel looks (MILESTONES M7).
const (
	expiryHorizonDays   = 30
	warrantyHorizonDays = 60
	maxSnoozeDays       = 365
)

// alertCandidate is the part of an item the alert rules read.
type alertCandidate struct {
	itemID          uuid.UUID
	name            string
	categoryID      uuid.UUID
	quantity        float64
	minQuantity     *float64
	unit            *string
	effectiveExpiry *time.Time
	warrantyUntil   *time.Time
}

// classify returns the alerts one item raises on the given day (a UTC
// midnight). An item can raise several: an expired item that is also out of
// stock appears under both kinds.
func classify(c *alertCandidate, today time.Time) []api.Alert {
	var out []api.Alert

	base := api.Alert{
		ItemId: c.itemID, ItemName: c.name, CategoryId: c.categoryID, Unit: c.unit,
	}

	if e := c.effectiveExpiry; e != nil {
		switch {
		case e.Before(today):
			out = append(out, withDue(base, api.AlertKindExpired, *e))
		case !e.After(today.AddDate(0, 0, expiryHorizonDays)):
			out = append(out, withDue(base, api.AlertKindExpiring, *e))
		}
	}

	if c.minQuantity != nil && c.quantity < *c.minQuantity {
		a := base
		a.Kind = api.AlertKindBelowMinimum
		qty, minQty := float32(c.quantity), float32(*c.minQuantity)
		a.Quantity, a.MinQuantity = &qty, &minQty
		out = append(out, a)
	}

	if w := c.warrantyUntil; w != nil && !w.Before(today) && !w.After(today.AddDate(0, 0, warrantyHorizonDays)) {
		out = append(out, withDue(base, api.AlertKindWarrantyEnding, *w))
	}

	return out
}

func withDue(a api.Alert, kind api.AlertKind, due time.Time) api.Alert {
	a.Kind = kind
	a.DueDate = &openapi_types.Date{Time: due}

	return a
}

// alertRank orders kinds by urgency in the panel.
var alertRank = map[api.AlertKind]int{
	api.AlertKindExpired:        0,
	api.AlertKindExpiring:       1,
	api.AlertKindBelowMinimum:   2,
	api.AlertKindWarrantyEnding: 3,
}

// utcDay truncates a moment to the UTC calendar day it falls in.
func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ListAlerts computes what needs attention today. Snoozed alerts are left
// out of Alerts and Counts and reported in Snoozed instead, so the UI can
// offer to bring them back.
func (s *Service) ListAlerts(ctx context.Context, principal auth.Principal) (api.AlertList, error) {
	familyID := pgconv.UUID(principal.FamilyID)
	today := utcDay(s.now())

	rows, err := s.store.ListAlertCandidates(ctx, db.ListAlertCandidatesParams{
		FamilyID:        familyID,
		Today:           pgconv.Date(&today),
		ExpiryHorizon:   dateAfter(today, expiryHorizonDays),
		WarrantyHorizon: dateAfter(today, warrantyHorizonDays),
	})
	if err != nil {
		return api.AlertList{}, fmt.Errorf("list alert candidates: %w", err)
	}

	dismissals, err := s.store.ListActiveAlertDismissals(ctx, db.ListActiveAlertDismissalsParams{
		FamilyID: familyID, Today: pgconv.Date(&today),
	})
	if err != nil {
		return api.AlertList{}, fmt.Errorf("list alert dismissals: %w", err)
	}

	type key struct {
		item uuid.UUID
		kind api.AlertKind
	}

	snoozed := make(map[key]db.AlertDismissal, len(dismissals))
	for i := range dismissals {
		snoozed[key{pgconv.ToUUID(dismissals[i].ItemID), api.AlertKind(dismissals[i].Kind)}] = dismissals[i]
	}

	out := api.AlertList{Alerts: []api.Alert{}, Counts: map[string]int32{}}
	hidden := []api.AlertDismissal{}

	for i := range rows {
		c := candidateFromRow(&rows[i])

		for _, a := range classify(&c, today) {
			if d, ok := snoozed[key{a.ItemId, a.Kind}]; ok {
				named := dismissalToAPI(&d)
				named.ItemName = &c.name
				hidden = append(hidden, named)

				continue
			}

			out.Alerts = append(out.Alerts, a)
			out.Counts[string(a.Kind)]++
		}
	}

	sortAlerts(out.Alerts)
	out.Snoozed = &hidden

	return out, nil
}

func sortAlerts(alerts []api.Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		a, b := alerts[i], alerts[j]

		if alertRank[a.Kind] != alertRank[b.Kind] {
			return alertRank[a.Kind] < alertRank[b.Kind]
		}

		if a.DueDate != nil && b.DueDate != nil && !a.DueDate.Equal(b.DueDate.Time) {
			return a.DueDate.Before(b.DueDate.Time)
		}

		return a.ItemName < b.ItemName
	})
}

func dateAfter(day time.Time, days int) pgtype.Date {
	t := day.AddDate(0, 0, days)

	return pgconv.Date(&t)
}

func candidateFromRow(r *db.ListAlertCandidatesRow) alertCandidate {
	return alertCandidate{
		itemID:          pgconv.ToUUID(r.ID),
		name:            r.Name,
		categoryID:      pgconv.ToUUID(r.CategoryID),
		quantity:        pgconv.ToNumeric(r.Quantity),
		minQuantity:     pgconv.ToNumericPtr(r.MinQuantity),
		unit:            pgconv.ToTextPtr(r.Unit),
		effectiveExpiry: pgconv.ToDatePtr(r.EffectiveExpiry),
		warrantyUntil:   pgconv.ToDatePtr(r.WarrantyUntil),
	}
}

func dismissalToAPI(d *db.AlertDismissal) api.AlertDismissal {
	return api.AlertDismissal{
		Id:           pgconv.ToUUID(d.ID),
		ItemId:       pgconv.ToUUID(d.ItemID),
		Kind:         api.AlertKind(d.Kind),
		SnoozedUntil: openapi_types.Date{Time: d.SnoozedUntil.Time},
	}
}

// DismissAlert snoozes one (item, kind) alert for the whole family. A
// repeat call extends or shortens the existing snooze.
func (s *Service) DismissAlert(
	ctx context.Context, principal auth.Principal, body api.DismissAlertRequest,
) (api.AlertDismissal, error) {
	if !body.Kind.Valid() {
		return api.AlertDismissal{}, &ValidationError{Msg: "unknown alert kind"}
	}

	if body.SnoozeDays < 1 || body.SnoozeDays > maxSnoozeDays {
		return api.AlertDismissal{}, &ValidationError{Msg: fmt.Sprintf("snoozeDays must be between 1 and %d", maxSnoozeDays)}
	}

	until := utcDay(s.now()).AddDate(0, 0, int(body.SnoozeDays))

	row, err := s.store.UpsertAlertDismissal(ctx, db.UpsertAlertDismissalParams{
		FamilyID:     pgconv.UUID(principal.FamilyID),
		ItemID:       pgconv.UUID(body.ItemId),
		Kind:         string(body.Kind),
		SnoozedUntil: pgconv.Date(&until),
		DismissedBy:  pgconv.UUID(principal.UserID),
	})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.AlertDismissal{}, wrapped
	}

	return dismissalToAPI(&row), nil
}

// UndoAlertDismissal brings a snoozed alert back.
func (s *Service) UndoAlertDismissal(ctx context.Context, principal auth.Principal, dismissalID uuid.UUID) error {
	affected, err := s.store.DeleteAlertDismissal(ctx, db.DeleteAlertDismissalParams{
		ID: pgconv.UUID(dismissalID), FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return fmt.Errorf("delete alert dismissal: %w", err)
	}

	if affected == 0 {
		return &NotFoundError{What: "dismissal"}
	}

	return nil
}

// ShoppingList is every active item below its minimum quantity.
func (s *Service) ShoppingList(ctx context.Context, principal auth.Principal) (api.ShoppingList, error) {
	rows, err := s.store.ListShoppingItems(ctx, pgconv.UUID(principal.FamilyID))
	if err != nil {
		return api.ShoppingList{}, fmt.Errorf("list shopping items: %w", err)
	}

	out := api.ShoppingList{Entries: make([]api.ShoppingListEntry, 0, len(rows))}

	for i := range rows {
		qty := float32(pgconv.ToNumeric(rows[i].Quantity))
		minQty := float32(pgconv.ToNumeric(rows[i].MinQuantity))
		short := minQty - qty

		out.Entries = append(out.Entries, api.ShoppingListEntry{
			ItemId: pgconv.ToUUID(rows[i].ID), Name: rows[i].Name,
			Quantity: qty, MinQuantity: minQty, Shortfall: &short,
			Unit: pgconv.ToTextPtr(rows[i].Unit),
		})
	}

	return out, nil
}
