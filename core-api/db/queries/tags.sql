-- Tags: family-defined labels an item can carry (SPEC 4, "Items").
--
-- No archived_at on `tags` itself — SPEC's own schema sketch omits it for
-- this one table, so deleting a tag is a genuine hard delete; `item_tags`
-- rows pointing at it cascade away with it (00008_items.sql).

-- name: CreateTag :one
INSERT INTO tags (family_id, name, color)
VALUES (sqlc.arg(family_id), sqlc.arg(name), sqlc.narg(color))
RETURNING *;

-- name: GetTag :one
SELECT *
FROM tags
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: ListTags :many
SELECT *
FROM tags
WHERE family_id = sqlc.arg(family_id)
ORDER BY name, id;

-- name: UpdateTag :one
UPDATE tags
SET name  = coalesce(sqlc.narg(name), name),
    color = coalesce(sqlc.narg(color), color)
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
RETURNING *;

-- name: DeleteTag :execrows
DELETE FROM tags
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: AttachItemTag :exec
-- Guarded by EXISTS against both items and tags so neither side can belong
-- to another family — the FK alone only proves the rows exist, not that
-- they are this family's own.
INSERT INTO item_tags (item_id, tag_id)
SELECT sqlc.arg(item_id), sqlc.arg(tag_id)
WHERE EXISTS (SELECT 1 FROM items i WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id))
  AND EXISTS (SELECT 1 FROM tags t WHERE t.id = sqlc.arg(tag_id) AND t.family_id = sqlc.arg(family_id))
ON CONFLICT DO NOTHING;

-- name: DetachItemTag :execrows
DELETE FROM item_tags
WHERE item_id = sqlc.arg(item_id)
  AND tag_id = sqlc.arg(tag_id)
  AND EXISTS (SELECT 1 FROM items i WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id));

-- name: ListTagsForItem :many
SELECT t.*
FROM tags t
         JOIN item_tags it ON it.tag_id = t.id
WHERE it.item_id = sqlc.arg(item_id)
  AND t.family_id = sqlc.arg(family_id)
ORDER BY t.name, t.id;
