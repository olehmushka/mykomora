-- Item events: the audit trail (SPEC 4, "Items" and SPEC 7, "Data safety").
--
-- Always written by internal/items' Service inside the same transaction as
-- the item write it describes — never as a best-effort side call. `type` is
-- plain text (see 00008_items.sql's header); the handful of values this
-- milestone writes are "created", "updated", "moved" and "status_changed".

-- name: CreateItemEvent :one
INSERT INTO item_events (family_id, item_id, actor_user_id, type, payload)
VALUES (sqlc.arg(family_id), sqlc.arg(item_id), sqlc.arg(actor_user_id), sqlc.arg(type), sqlc.arg(payload))
RETURNING *;

-- name: ListItemEvents :many
SELECT *
FROM item_events
WHERE item_id = sqlc.arg(item_id)
  AND family_id = sqlc.arg(family_id)
ORDER BY created_at DESC, id
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
