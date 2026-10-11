-- Item attachments: receipts, manuals, warranty documents (SPEC 4, "Items";
-- SPEC 6, "Photos and attachments").
--
-- Like item_photos (db/queries/photos.sql), item_attachments has no
-- family_id of its own; every query here scopes through items.

-- name: CreateItemAttachment :one
INSERT INTO item_attachments (item_id, kind, storage_key, filename, content_type, bytes, uploaded_by)
SELECT sqlc.arg(item_id), sqlc.arg(kind), sqlc.arg(storage_key), sqlc.arg(filename),
       sqlc.arg(content_type), sqlc.arg(bytes), sqlc.arg(uploaded_by)
WHERE EXISTS (SELECT 1 FROM items i WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id))
RETURNING *;

-- name: ListItemAttachments :many
SELECT a.*
FROM item_attachments a
         JOIN items i ON i.id = a.item_id
WHERE a.item_id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id)
ORDER BY a.created_at, a.id;

-- name: DeleteItemAttachment :one
-- Returns the deleted row so the caller can delete its object from storage.
DELETE FROM item_attachments
WHERE item_attachments.id = sqlc.arg(id)
  AND item_attachments.item_id = sqlc.arg(item_id)
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = item_attachments.item_id AND i.family_id = sqlc.arg(family_id))
RETURNING *;

-- name: ListAllStorageKeys :many
-- Cross-tenant on purpose: the orphaned-object cleanup sweeper (internal/
-- storage/cleanup.go) has no per-family loop to run, it just needs every
-- key any family's rows still reference, to diff against what the bucket
-- actually holds. Allowlisted in sqlc.yaml's family-scoped rule for exactly
-- this reason, the same way the refresh-token reuse-detection revoke is.
SELECT storage_key FROM item_photos
UNION ALL
SELECT thumb_key FROM item_photos
UNION ALL
SELECT storage_key FROM item_attachments;
