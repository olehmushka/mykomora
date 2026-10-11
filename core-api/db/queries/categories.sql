-- Categories: the built-in taxonomy plus each family's own (SPEC 4,
-- "Taxonomy").
--
-- family_id NULL means built-in — its label is resolved from the Go message
-- catalogue via name_i18n_key. family_id set means family-defined — its label
-- is name_custom. Reads return both; writes can only ever touch a family's
-- own rows, because `family_id = sqlc.arg(family_id)` never matches a NULL
-- row. Built-ins exist solely via the seed migration (00006) — there is no
-- create/update/archive path for them.

-- name: ListCategories :many
SELECT *
FROM categories
WHERE (family_id = sqlc.arg(family_id) OR family_id IS NULL)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY parent_id NULLS FIRST, sort_order, slug;

-- name: GetCategory :one
SELECT *
FROM categories
WHERE id = sqlc.arg(id)
  AND (family_id = sqlc.arg(family_id) OR family_id IS NULL);

-- name: CreateCategory :one
-- Guarded so a family cannot point parent_id at a category it cannot see:
-- the FK alone only proves the row exists, not that it is this family's own
-- or a built-in.
INSERT INTO categories (family_id, parent_id, slug, icon, name_custom, sort_order)
SELECT sqlc.arg(family_id), sqlc.narg(parent_id), sqlc.arg(slug), sqlc.narg(icon),
       sqlc.arg(name_custom), sqlc.arg(sort_order)
WHERE sqlc.narg(parent_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM categories pc
    WHERE pc.id = sqlc.narg(parent_id) AND (pc.family_id = sqlc.arg(family_id) OR pc.family_id IS NULL)
)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories
SET name_custom = coalesce(sqlc.narg(name_custom), name_custom),
    icon         = coalesce(sqlc.narg(icon), icon),
    sort_order   = coalesce(sqlc.narg(sort_order), sort_order),
    updated_at   = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveCategory :execrows
UPDATE categories
SET archived_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL;
