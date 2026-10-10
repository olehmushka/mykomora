-- Trait definitions: the typed, category-specific fields a category carries
-- (SPEC 4, "Taxonomy").
--
-- Same family_id-nullable shape as categories.sql: built-in rows (family_id
-- NULL) come from the seed migration; a custom field is a family-defined row
-- that may attach to either a built-in or a family-defined category — "one
-- code path, one validator, one renderer" applies per field, not per category.

-- name: ListTraitDefsForCategories :many
-- Driven by a category id list so GET /taxonomy issues one query, not N+1.
SELECT *
FROM trait_defs
WHERE category_id = ANY (sqlc.arg(category_ids)::uuid[])
  AND (family_id = sqlc.arg(family_id) OR family_id IS NULL)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY sort_order, key;

-- name: GetTraitDef :one
SELECT *
FROM trait_defs
WHERE id = sqlc.arg(id)
  AND (family_id = sqlc.arg(family_id) OR family_id IS NULL);

-- name: CreateTraitDef :one
-- Guarded by EXISTS against categories so a custom field can only attach to a
-- category this family can actually see (its own, or a built-in).
INSERT INTO trait_defs (category_id, family_id, key, data_type, unit, enum_options,
                         label_custom, required, filterable, sort_order)
SELECT sqlc.arg(category_id), sqlc.arg(family_id), sqlc.arg(key), sqlc.arg(data_type),
       sqlc.narg(unit), sqlc.narg(enum_options), sqlc.arg(label_custom),
       sqlc.arg(required), sqlc.arg(filterable), sqlc.arg(sort_order)
WHERE EXISTS (
    SELECT 1 FROM categories c
    WHERE c.id = sqlc.arg(category_id)
      AND (c.family_id = sqlc.arg(family_id) OR c.family_id IS NULL)
)
RETURNING *;

-- name: UpdateTraitDef :one
UPDATE trait_defs
SET label_custom = coalesce(sqlc.narg(label_custom), label_custom),
    required     = coalesce(sqlc.narg(required), required),
    filterable   = coalesce(sqlc.narg(filterable), filterable),
    sort_order   = coalesce(sqlc.narg(sort_order), sort_order),
    updated_at   = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveTraitDef :execrows
UPDATE trait_defs
SET archived_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL;
