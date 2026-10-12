package items

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// AdjustQuantity moves a consumable's quantity by a delta, or sets it
// outright, clamping at zero, and records a quantity_changed event in the
// same transaction. An adjustment that changes nothing (a no-op set, or a
// decrement at zero) is accepted but writes no event.
func (s *Service) AdjustQuantity(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.AdjustQuantityRequest,
) (api.Item, error) {
	if (body.Delta == nil) == (body.Quantity == nil) {
		return api.Item{}, &ValidationError{Msg: "send exactly one of delta or quantity"}
	}

	if body.Quantity != nil && *body.Quantity < 0 {
		return api.Item{}, &ValidationError{Msg: "quantity must not be negative"}
	}

	familyID := pgconv.UUID(principal.FamilyID)
	params := db.AdjustItemQuantityParams{
		NewQuantity: pgconv.NumericPtr(float64Ptr(body.Quantity)),
		Delta:       pgconv.NumericPtr(float64Ptr(body.Delta)),
		ID:          pgconv.UUID(itemID),
		FamilyID:    familyID,
	}

	var updated db.Item

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.AdjustItemQuantity(ctx, params)
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		updated = itemFromAdjustRow(&row)

		from, to := pgconv.ToNumeric(row.PreviousQuantity), pgconv.ToNumeric(row.Quantity)
		if math.Abs(from-to) < quantityEpsilon {
			return nil
		}

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: row.ID, ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventQuantityChanged, Payload: pgconv.JSONB(quantityChangedPayload(from, to)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})
	if err != nil {
		return api.Item{}, err
	}

	return s.toAPIItem(ctx, principal.FamilyID, &updated)
}

// quantityEpsilon is below any quantity a person can type, so two numerics
// closer than this are the same quantity.
const quantityEpsilon = 1e-9

// itemFromAdjustRow drops the extra previous_quantity column sqlc adds to
// the row type of a query that RETURNs items.* plus something else.
func itemFromAdjustRow(r *db.AdjustItemQuantityRow) db.Item {
	return db.Item{
		ID: r.ID, FamilyID: r.FamilyID, Name: r.Name, CategoryID: r.CategoryID,
		LocationID: r.LocationID, RoomID: r.RoomID, Spot: r.Spot,
		ParentItemID: r.ParentItemID, IsContainer: r.IsContainer, Status: r.Status,
		Quantity: r.Quantity, Unit: r.Unit, MinQuantity: r.MinQuantity,
		ExpiresAt: r.ExpiresAt, OpenedAt: r.OpenedAt, PaoMonths: r.PaoMonths,
		Brand: r.Brand, Condition: r.Condition, Season: r.Season,
		SizeSystem: r.SizeSystem, SizeValue: r.SizeValue,
		SizeNormMin: r.SizeNormMin, SizeNormMax: r.SizeNormMax,
		ForPersonID: r.ForPersonID, SerialNumber: r.SerialNumber,
		WarrantyUntil: r.WarrantyUntil, PurchaseDate: r.PurchaseDate,
		PurchasePrice: r.PurchasePrice, PurchaseCurrency: r.PurchaseCurrency,
		Notes: r.Notes, Traits: r.Traits, CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt,
		SearchTsv: r.SearchTsv,
	}
}
