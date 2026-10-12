-- Kids' sizes and seasons (M8; SPEC 5, "Kids: what fits now, what's next").
--
-- `person_sizes` has no family_id of its own (SPEC 4); like rooms, it scopes
-- through its parent, `people`, so the caller proves the person is theirs.
-- The size_norm_* columns are filled by the person_sizes_normalise trigger.

-- name: ListPersonSizes :many
SELECT ps.*
FROM person_sizes ps
         JOIN people p ON p.id = ps.person_id
WHERE ps.person_id = sqlc.arg(person_id)
  AND p.family_id = sqlc.arg(family_id)
ORDER BY ps.recorded_at DESC, ps.id;

-- name: CreatePersonSize :one
-- Written as INSERT ... SELECT so a person from another family (or an
-- archived one) inserts nothing and surfaces as no rows.
INSERT INTO person_sizes (person_id, size_type, size_system, size_value)
SELECT p.id, sqlc.arg(size_type), sqlc.arg(size_system), sqlc.arg(size_value)
FROM people p
WHERE p.id = sqlc.arg(person_id)
  AND p.family_id = sqlc.arg(family_id)
  AND p.archived_at IS NULL
RETURNING *;

-- name: ListCurrentPersonSizes :many
-- The newest recorded size per size_type.
SELECT DISTINCT ON (ps.size_type) ps.*
FROM person_sizes ps
         JOIN people p ON p.id = ps.person_id
WHERE ps.person_id = sqlc.arg(person_id)
  AND p.family_id = sqlc.arg(family_id)
ORDER BY ps.size_type, ps.recorded_at DESC, ps.id;

-- name: ListFitItems :many
-- Items whose normalised size range overlaps [range_min, range_max] (strictly:
-- touching at a bracket boundary is not a fit). An item is shoe-sized when its
-- size_system is a shoe_* one and clothing-sized otherwise; items already
-- earmarked for somebody else are left out, and so are items already on their
-- way out (to_give_away) or gone.
SELECT sqlc.embed(items), ph.thumb_key AS primary_thumb_key
FROM items
         LEFT JOIN item_photos ph ON ph.item_id = items.id AND ph.is_primary
WHERE items.family_id = sqlc.arg(family_id)
  AND items.archived_at IS NULL
  AND items.status IN ('in_use', 'stored')
  AND items.size_norm_min IS NOT NULL
  AND items.size_norm_min < sqlc.arg(range_max)::numeric
  AND items.size_norm_max > sqlc.arg(range_min)::numeric
  AND (CASE WHEN items.size_system::text LIKE 'shoe\_%' THEN 'shoe' ELSE 'clothing' END) = sqlc.arg(size_type)::text
  AND (items.for_person_id IS NULL OR items.for_person_id = sqlc.arg(person_id))
ORDER BY items.name, items.id
LIMIT sqlc.arg(result_limit);

-- name: ListBatchEvents :many
SELECT *
FROM item_events
WHERE family_id = sqlc.arg(family_id)
  AND batch_id = sqlc.arg(batch_id)
  AND type = 'status_changed'
ORDER BY created_at, id;
