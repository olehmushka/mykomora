-- Item photos (SPEC 4, "Items"; SPEC 6, "Photos and attachments").
--
-- item_photos has no family_id column of its own — its tenant key lives one
-- hop away, through item_id, the same shape rooms.sql established for
-- rooms-through-locations. Every query here scopes through items rather
-- than taking a vet-rule exemption (CLAUDE.md, "Every query is family-scoped").

-- name: CreateItemPhoto :one
-- is_primary is decided by the caller (the first photo on an item is always
-- primary; internal/items.Service counts existing photos before calling
-- this) rather than computed here, the same preference for Go-level logic
-- over a cleverer INSERT that CreateItem/MoveItem already established.
INSERT INTO item_photos (item_id, storage_key, thumb_key, width, height, bytes, content_type, is_primary, sort_order, uploaded_by)
SELECT sqlc.arg(item_id), sqlc.arg(storage_key), sqlc.arg(thumb_key), sqlc.arg(width), sqlc.arg(height),
       sqlc.arg(bytes), sqlc.arg(content_type), sqlc.arg(is_primary), sqlc.arg(sort_order), sqlc.arg(uploaded_by)
WHERE EXISTS (SELECT 1 FROM items i WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id))
RETURNING *;

-- name: GetItemPhoto :one
SELECT p.*
FROM item_photos p
         JOIN items i ON i.id = p.item_id
WHERE p.id = sqlc.arg(id)
  AND p.item_id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id);

-- name: ListItemPhotos :many
SELECT p.*
FROM item_photos p
         JOIN items i ON i.id = p.item_id
WHERE p.item_id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id)
ORDER BY p.sort_order, p.id;

-- name: CountItemPhotos :one
SELECT count(*)
FROM item_photos p
         JOIN items i ON i.id = p.item_id
WHERE p.item_id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id);

-- name: ClearItemPhotoPrimary :exec
-- Unsets any other primary photo on the item before a new one is promoted —
-- the partial unique index on (item_id) WHERE is_primary allows at most one.
UPDATE item_photos
SET is_primary = false
WHERE item_photos.item_id = sqlc.arg(item_id)
  AND item_photos.id != sqlc.arg(id)
  AND item_photos.is_primary
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = item_photos.item_id AND i.family_id = sqlc.arg(family_id));

-- name: UpdateItemPhoto :one
-- A NULL argument leaves its column untouched. Setting is_primary is always
-- preceded by ClearItemPhotoPrimary in the same transaction when promoting a
-- new primary.
UPDATE item_photos
SET is_primary = coalesce(sqlc.narg(is_primary), is_primary),
    sort_order = coalesce(sqlc.narg(sort_order), sort_order)
WHERE item_photos.id = sqlc.arg(id)
  AND item_photos.item_id = sqlc.arg(item_id)
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = item_photos.item_id AND i.family_id = sqlc.arg(family_id))
RETURNING *;

-- name: DeleteItemPhoto :one
-- Returns the deleted row so the caller can delete its two objects from
-- storage and know whether it was primary (to promote the next one).
DELETE FROM item_photos
WHERE item_photos.id = sqlc.arg(id)
  AND item_photos.item_id = sqlc.arg(item_id)
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = item_photos.item_id AND i.family_id = sqlc.arg(family_id))
RETURNING *;

-- name: PromoteNextItemPhoto :one
-- Called after deleting a primary photo that still has siblings: makes the
-- lowest-sort_order survivor the new primary.
UPDATE item_photos
SET is_primary = true
WHERE item_photos.id = (
    SELECT p.id FROM item_photos p
    WHERE p.item_id = sqlc.arg(item_id)
    ORDER BY p.sort_order, p.id
    LIMIT 1
)
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = item_photos.item_id AND i.family_id = sqlc.arg(family_id))
RETURNING *;
