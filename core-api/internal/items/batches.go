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

// Bulk status actions (the "outgrown" sweep and the seasonal pack/unpack).
// Each writes one status_changed event per item, all carrying the same
// batch_id, in a single transaction; UndoBatch uses that id to reverse them.

// BulkSetStatus moves many items to one active status. Items already there,
// or already archived, are counted as skipped.
func (s *Service) BulkSetStatus(
	ctx context.Context, principal auth.Principal, body api.BulkSetItemStatusRequest,
) (api.BatchResult, error) {
	if len(body.ItemIds) == 0 {
		return api.BatchResult{}, &ValidationError{Msg: "choose at least one item"}
	}

	if !IsActive(body.Status) {
		return api.BatchResult{}, &ValidationError{Msg: "bulk changes only accept active statuses"}
	}

	ids := make([]uuid.UUID, 0, len(body.ItemIds))
	seen := make(map[uuid.UUID]bool, len(body.ItemIds))

	for _, id := range body.ItemIds {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	familyID := pgconv.UUID(principal.FamilyID)
	result := api.BatchResult{BatchId: uuid.New()}

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		for _, id := range ids {
			item, err := q.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(id), FamilyID: familyID})
			if wrapped := wrapNotFound(err, "item"); wrapped != nil {
				return wrapped
			}

			changed, err := moveToStatus(ctx, q, principal, &item, body.Status, result.BatchId)
			if err != nil {
				return err
			}

			tally(&result, changed)
		}

		return nil
	})

	return result, err
}

// FlipSeason packs (in_use → stored) or unpacks (stored → in_use) the direct
// contents of a container.
func (s *Service) FlipSeason(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.SeasonFlipRequest,
) (api.BatchResult, error) {
	from, to := api.InUse, api.Stored
	if body.Direction == api.Unpack {
		from, to = api.Stored, api.InUse
	}

	familyID := pgconv.UUID(principal.FamilyID)
	result := api.BatchResult{BatchId: uuid.New()}

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		container, err := q.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		if !container.IsContainer {
			return &ValidationError{Msg: "only a container can be packed or unpacked"}
		}

		children, err := q.ListItemChildren(ctx, db.ListItemChildrenParams{
			FamilyID: familyID, ParentItemID: container.ID,
		})
		if err != nil {
			return fmt.Errorf("list contents: %w", err)
		}

		for i := range children {
			if api.ItemStatus(children[i].Status) != from {
				result.Skipped++

				continue
			}

			changed, err := moveToStatus(ctx, q, principal, &children[i], to, result.BatchId)
			if err != nil {
				return err
			}

			tally(&result, changed)
		}

		return nil
	})

	return result, err
}

// UndoBatch restores every item a batch changed to its previous status, as
// long as it still has the status the batch gave it.
func (s *Service) UndoBatch(ctx context.Context, principal auth.Principal, batchID uuid.UUID) (api.BatchResult, error) {
	familyID := pgconv.UUID(principal.FamilyID)
	result := api.BatchResult{BatchId: batchID}

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		events, err := q.ListBatchEvents(ctx, db.ListBatchEventsParams{FamilyID: familyID, BatchID: pgconv.UUID(batchID)})
		if err != nil {
			return fmt.Errorf("list batch events: %w", err)
		}

		if len(events) == 0 {
			return &NotFoundError{What: "batch"}
		}

		for i := range events {
			payload := pgconv.ToJSONB(events[i].Payload)
			from, to := stringField(payload, "from"), stringField(payload, "to")

			item, err := q.GetItem(ctx, db.GetItemParams{ID: events[i].ItemID, FamilyID: familyID})
			if err != nil {
				return fmt.Errorf("load item: %w", err)
			}

			if item.ArchivedAt.Valid || string(item.Status) != to {
				result.Skipped++

				continue
			}

			if err := writeStatus(ctx, q, principal, &item, api.ItemStatus(from), pgtype.UUID{}, map[string]any{"undoOf": batchID.String()}); err != nil {
				return err
			}

			result.Changed++
		}

		return nil
	})

	return result, err
}

// moveToStatus applies a status to one item inside a batch. It reports false
// for an item that was left alone (archived, or already there).
func moveToStatus(
	ctx context.Context, q *db.Queries, principal auth.Principal, item *db.Item, to api.ItemStatus, batchID uuid.UUID,
) (bool, error) {
	if item.ArchivedAt.Valid || api.ItemStatus(item.Status) == to {
		return false, nil
	}

	if !CanTransition(api.ItemStatus(item.Status), to) {
		return false, nil
	}

	if err := writeStatus(ctx, q, principal, item, to, pgconv.UUID(batchID), nil); err != nil {
		return false, err
	}

	return true, nil
}

func writeStatus(
	ctx context.Context, q *db.Queries, principal auth.Principal, item *db.Item,
	to api.ItemStatus, batchID pgtype.UUID, extra map[string]any,
) error {
	if _, err := q.SetItemStatus(ctx, db.SetItemStatusParams{
		Status: db.ItemStatus(to), ID: item.ID, FamilyID: item.FamilyID,
	}); err != nil {
		return fmt.Errorf("set item status: %w", err)
	}

	payload := statusChangedPayload(api.ItemStatus(item.Status), to)
	for k, v := range extra {
		payload[k] = v
	}

	if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
		FamilyID: item.FamilyID, ItemID: item.ID, ActorUserID: pgconv.UUID(principal.UserID),
		Type: eventStatusChanged, Payload: pgconv.JSONB(payload), BatchID: batchID,
	}); err != nil {
		return fmt.Errorf("create item event: %w", err)
	}

	return nil
}

func tally(r *api.BatchResult, changed bool) {
	if changed {
		r.Changed++
	} else {
		r.Skipped++
	}
}

// stringField reads a string out of an event payload, empty if it is absent
// or not a string.
func stringField(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}

	return ""
}
