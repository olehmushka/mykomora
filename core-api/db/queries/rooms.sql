-- Rooms: inside a location (SPEC 4, "Places").
--
-- `rooms` has no family_id column of its own — like `families`, its tenant
-- key lives one hop away. Every query here scopes through `locations` rather
-- than taking a vet-rule exemption (CLAUDE.md, "Every query is family-scoped"),
-- replicating the pattern families.sql established for GetFamily/UpdateFamily.
-- Get this join wrong and a room leaks across families with no complaint from
-- the vet rule, which only checks for the literal text "family_id" — not that
-- it actually constrains the right row.

-- name: CreateRoom :one
-- SELECT ... WHERE EXISTS, not a plain INSERT: the row is only written if the
-- location is actually this family's, which is what proves the room may be
-- created here at all.
INSERT INTO rooms (location_id, name, sort_order)
SELECT sqlc.arg(location_id), sqlc.arg(name), sqlc.arg(sort_order)
WHERE EXISTS (
    SELECT 1 FROM locations l
    WHERE l.id = sqlc.arg(location_id) AND l.family_id = sqlc.arg(family_id)
)
RETURNING *;

-- name: GetRoom :one
SELECT r.*
FROM rooms r
         JOIN locations l ON l.id = r.location_id
WHERE r.id = sqlc.arg(id)
  AND l.family_id = sqlc.arg(family_id);

-- name: ListRoomsByLocation :many
SELECT r.*
FROM rooms r
         JOIN locations l ON l.id = r.location_id
WHERE r.location_id = sqlc.arg(location_id)
  AND l.family_id = sqlc.arg(family_id)
  AND (sqlc.arg(include_archived)::boolean OR r.archived_at IS NULL)
ORDER BY r.sort_order, r.name, r.id;

-- name: UpdateRoom :one
UPDATE rooms
SET name       = coalesce(sqlc.narg(name), name),
    sort_order = coalesce(sqlc.narg(sort_order), sort_order),
    updated_at = now()
WHERE rooms.id = sqlc.arg(id)
  AND archived_at IS NULL
  AND EXISTS (
    SELECT 1 FROM locations l
    WHERE l.id = rooms.location_id AND l.family_id = sqlc.arg(family_id)
)
RETURNING *;

-- name: ArchiveRoom :execrows
UPDATE rooms
SET archived_at = now(),
    updated_at  = now()
WHERE rooms.id = sqlc.arg(id)
  AND archived_at IS NULL
  AND EXISTS (
    SELECT 1 FROM locations l
    WHERE l.id = rooms.location_id AND l.family_id = sqlc.arg(family_id)
);
