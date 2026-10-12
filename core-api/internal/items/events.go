package items

import (
	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Event types this and earlier milestones write. Later milestones add more
// (SPEC 4, "Items"; 00008_items.sql's header note on item_events.type being
// plain text for exactly this reason).
const (
	eventCreated           = "created"
	eventUpdated           = "updated"
	eventMoved             = "moved"
	eventStatusChanged     = "status_changed"
	eventArchived          = "archived"
	eventPhotoAdded        = "photo_added"
	eventPhotoRemoved      = "photo_removed"
	eventAttachmentAdded   = "attachment_added"
	eventAttachmentRemoved = "attachment_removed"
	eventQuantityChanged   = "quantity_changed"
	eventLent              = "lent"
	eventReturned          = "returned"
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

// photoAddedPayload records which photo was added and whether it became
// primary — not its storage keys, which are an internal detail of where
// the bytes live rather than part of the item's history.
func photoAddedPayload(photo *db.ItemPhoto) map[string]any {
	return map[string]any{
		"photoId":   pgconv.ToUUID(photo.ID).String(),
		"isPrimary": photo.IsPrimary,
	}
}

// photoRemovedPayload records which photo was removed, and the photo that
// was promoted to replace it as primary, if any.
func photoRemovedPayload(removed, promoted *db.ItemPhoto) map[string]any {
	payload := map[string]any{"photoId": pgconv.ToUUID(removed.ID).String()}
	if promoted != nil {
		payload["promotedPhotoId"] = pgconv.ToUUID(promoted.ID).String()
	}

	return payload
}

// attachmentAddedPayload records which attachment was added and its kind.
func attachmentAddedPayload(attachment *db.ItemAttachment) map[string]any {
	return map[string]any{
		"attachmentId": pgconv.ToUUID(attachment.ID).String(),
		"kind":         string(attachment.Kind),
	}
}

// attachmentRemovedPayload records which attachment was removed.
func attachmentRemovedPayload(attachment *db.ItemAttachment) map[string]any {
	return map[string]any{"attachmentId": pgconv.ToUUID(attachment.ID).String()}
}

// quantityChangedPayload records the before and after of a quick +/- or an
// absolute set, so the timeline can say "3 → 2" without diffing rows.
func quantityChangedPayload(from, to float64) map[string]any {
	return map[string]any{"from": from, "to": to}
}
