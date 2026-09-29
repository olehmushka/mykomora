-- People: anyone the family tracks things for (SPEC 3, "Concepts").
--
-- Archived rather than deleted, because items recorded against a person must
-- keep their history when that person stops being current.

-- name: CreatePerson :one
INSERT INTO people (family_id, name, birthdate, user_id, is_child, avatar_url)
VALUES (sqlc.arg(family_id), sqlc.arg(name), sqlc.narg(birthdate),
        sqlc.narg(user_id), sqlc.arg(is_child), sqlc.narg(avatar_url))
RETURNING *;

-- name: GetPerson :one
SELECT *
FROM people
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: ListPeople :many
-- `include_archived` is a parameter rather than a second query so the list and
-- its archived variant cannot drift apart.
SELECT *
FROM people
WHERE family_id = sqlc.arg(family_id)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY is_child DESC, name, id;

-- name: UpdatePerson :one
-- Partial update; a NULL argument leaves its column untouched. `birthdate`,
-- `user_id` and `avatar_url` are therefore not clearable through this path,
-- which is the right trade while nothing in the UI clears them.
UPDATE people
SET name       = coalesce(sqlc.narg(name), name),
    birthdate  = coalesce(sqlc.narg(birthdate), birthdate),
    user_id    = coalesce(sqlc.narg(user_id), user_id),
    is_child   = coalesce(sqlc.narg(is_child), is_child),
    avatar_url = coalesce(sqlc.narg(avatar_url), avatar_url),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchivePerson :execrows
UPDATE people
SET archived_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL;
