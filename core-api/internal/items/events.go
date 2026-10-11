package items

import (
	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Event types this milestone writes. Later milestones add more (SPEC 4,
// "Items"; 00008_items.sql's header note on item_events.type being plain
// text for exactly this reason).
const (
	eventCreated       = "created"
	eventUpdated       = "updated"
	eventMoved         = "moved"
	eventStatusChanged = "status_changed"
	eventArchived      = "archived"
)

// createdPayload records what the item looked like at birth.
func createdPayload(it *db.Item) map[string]any {
	return map[string]any{
		"categoryId": pgconv.ToUUID(it.CategoryID).String(),
		"locationId": pgconv.ToUUID(it.LocationID).String(),
		"status":     string(it.Status),
	}
}

// updatedPayload records which fields changed, not their values — the
// current row already has the values, and this keeps the event small.
func updatedPayload(changedFields []string) map[string]any {
	return map[string]any{"fields": changedFields}
}

// movedPayload records the place or container transition, whichever side
// of it applies.
func movedPayload(before, after *db.Item) map[string]any {
	return map[string]any{
		"from": placePayload(before),
		"to":   placePayload(after),
	}
}

func placePayload(it *db.Item) map[string]any {
	place := map[string]any{}

	if parentID := pgconv.ToUUIDPtr(it.ParentItemID); parentID != nil {
		place["parentItemId"] = parentID.String()

		return place
	}

	place["locationId"] = pgconv.ToUUID(it.LocationID).String()

	if roomID := pgconv.ToUUIDPtr(it.RoomID); roomID != nil {
		place["roomId"] = roomID.String()
	}

	if spot := pgconv.ToTextPtr(it.Spot); spot != nil {
		place["spot"] = *spot
	}

	return place
}

// statusChangedPayload records the transition itself.
func statusChangedPayload(from, to api.ItemStatus) map[string]any {
	return map[string]any{"from": string(from), "to": string(to)}
}
