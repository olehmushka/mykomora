-- Locations: one of the family's physical places (SPEC 3, "Concepts").
--
-- Scoped directly on family_id, same shape as people.sql.

-- name: CreateLocation :one
INSERT INTO locations (family_id, name, kind, address, sort_order)
VALUES (sqlc.arg(family_id), sqlc.arg(name), sqlc.arg(kind), sqlc.narg(address), sqlc.arg(sort_order))
RETURNING *;

-- name: GetLocation :one
SELECT *
FROM locations
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: ListLocations :many
SELECT *
FROM locations
WHERE family_id = sqlc.arg(family_id)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY sort_order, name, id;

-- name: UpdateLocation :one
-- Partial update; a NULL argument leaves its column untouched.
UPDATE locations
SET name       = coalesce(sqlc.narg(name), name),
    kind        = coalesce(sqlc.narg(kind), kind),
    address     = coalesce(sqlc.narg(address), address),
    sort_order  = coalesce(sqlc.narg(sort_order), sort_order),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveLocation :execrows
UPDATE locations
SET archived_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL;
