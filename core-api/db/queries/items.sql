-- Items: real possessions, in real places, optionally inside a container
-- (SPEC 3, "Concepts" and SPEC 4, "Items").
--
-- `items.family_id` is a real column, so every query here filters on it
-- directly, the same shape as locations.sql — no EXISTS-through-a-parent
-- needed, unlike rooms.sql.
--
-- Containment is read-resolved, not denormalised (see 00008_items.sql's
-- header): GetItemAncestorChain and GetItemDescendantDepths are the two
-- recursive queries internal/items builds cycle prevention, depth limits
-- and effective-place resolution on top of. Both still filter every row on
-- family_id, so a chain can never walk into another family's items even if
-- a caller somehow had a foreign id to try.

-- name: CreateItem :one
INSERT INTO items (
    family_id, name, category_id,
    location_id, room_id, spot,
    parent_item_id, is_container,
    status,
    quantity, unit, min_quantity, expires_at, opened_at, pao_months,
    brand, condition, season, size_system, size_value,
    for_person_id, serial_number, warranty_until,
    purchase_date, purchase_price, purchase_currency, notes,
    traits, created_by
) VALUES (
    sqlc.arg(family_id), sqlc.arg(name), sqlc.arg(category_id),
    sqlc.arg(location_id), sqlc.narg(room_id), sqlc.narg(spot),
    sqlc.narg(parent_item_id), sqlc.arg(is_container),
    sqlc.arg(status),
    sqlc.arg(quantity), sqlc.narg(unit), sqlc.narg(min_quantity),
    sqlc.narg(expires_at), sqlc.narg(opened_at), sqlc.narg(pao_months),
    sqlc.narg(brand), sqlc.narg(condition), sqlc.narg(season),
    sqlc.narg(size_system), sqlc.narg(size_value),
    sqlc.narg(for_person_id), sqlc.narg(serial_number), sqlc.narg(warranty_until),
    sqlc.narg(purchase_date), sqlc.narg(purchase_price), sqlc.narg(purchase_currency), sqlc.narg(notes),
    sqlc.arg(traits), sqlc.arg(created_by)
)
RETURNING *;

-- name: GetItem :one
SELECT *
FROM items
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: GetItemAncestorChain :many
-- Row 0 is the item itself; the last row is its root. Used for depth
-- computation, effective-place resolution (the root's place columns), and —
-- called with a candidate parent's id — cycle detection on re-parenting: if
-- the item being moved appears anywhere in the candidate's own chain, the
-- candidate is that item or one of its descendants.
WITH RECURSIVE chain AS (
    SELECT i.*, 0 AS depth
    FROM items i
    WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id)
    UNION ALL
    SELECT p.*, chain.depth + 1
    FROM items p
             JOIN chain ON p.id = chain.parent_item_id
    WHERE p.family_id = sqlc.arg(family_id)
)
SELECT * FROM chain ORDER BY depth;

-- name: GetItemDescendantDepths :many
-- Every existing descendant of an item, with its depth relative to that
-- item (the item itself is not included). Used only when moving a
-- container with contents, to check the deepest descendant still fits
-- under the max-depth-5 rule once the container is re-parented.
WITH RECURSIVE subtree AS (
    SELECT i.id, i.parent_item_id, 0 AS relative_depth
    FROM items i
    WHERE i.id = sqlc.arg(item_id) AND i.family_id = sqlc.arg(family_id)
    UNION ALL
    SELECT c.id, c.parent_item_id, subtree.relative_depth + 1
    FROM items c
             JOIN subtree ON c.parent_item_id = subtree.id
    WHERE c.family_id = sqlc.arg(family_id)
)
SELECT * FROM subtree WHERE relative_depth > 0 ORDER BY relative_depth DESC;

-- name: ListItems :many
-- Roots only (no parent): a nested item's place columns are cosmetic, so it
-- is reached by following a container's contents link, not by this list
-- (00008_items.sql's header). Filters are optional; an absent one matches
-- everything.
SELECT *
FROM items
WHERE family_id = sqlc.arg(family_id)
  AND parent_item_id IS NULL
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
  AND (sqlc.narg(location_id)::uuid IS NULL OR location_id = sqlc.narg(location_id))
  AND (sqlc.narg(room_id)::uuid IS NULL OR room_id = sqlc.narg(room_id))
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
ORDER BY created_at DESC, id
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: ListItemChildren :many
SELECT *
FROM items
WHERE family_id = sqlc.arg(family_id)
  AND parent_item_id = sqlc.arg(parent_item_id)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY name, id;

-- name: UpdateItem :one
-- Everything except place and status, which only the move and status
-- actions touch (so an `updated` event can never be confused with a
-- `moved` or `status_changed` one). A NULL argument leaves its column
-- untouched.
UPDATE items
SET name              = coalesce(sqlc.narg(name), name),
    category_id       = coalesce(sqlc.narg(category_id), category_id),
    quantity          = coalesce(sqlc.narg(quantity), quantity),
    unit              = coalesce(sqlc.narg(unit), unit),
    min_quantity      = coalesce(sqlc.narg(min_quantity), min_quantity),
    expires_at        = coalesce(sqlc.narg(expires_at), expires_at),
    opened_at         = coalesce(sqlc.narg(opened_at), opened_at),
    pao_months        = coalesce(sqlc.narg(pao_months), pao_months),
    brand             = coalesce(sqlc.narg(brand), brand),
    condition         = coalesce(sqlc.narg(condition), condition),
    season            = coalesce(sqlc.narg(season), season),
    size_system       = coalesce(sqlc.narg(size_system), size_system),
    size_value        = coalesce(sqlc.narg(size_value), size_value),
    for_person_id     = coalesce(sqlc.narg(for_person_id), for_person_id),
    serial_number     = coalesce(sqlc.narg(serial_number), serial_number),
    warranty_until    = coalesce(sqlc.narg(warranty_until), warranty_until),
    purchase_date     = coalesce(sqlc.narg(purchase_date), purchase_date),
    purchase_price    = coalesce(sqlc.narg(purchase_price), purchase_price),
    purchase_currency = coalesce(sqlc.narg(purchase_currency), purchase_currency),
    notes             = coalesce(sqlc.narg(notes), notes),
    traits            = coalesce(sqlc.narg(traits), traits),
    updated_at        = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: MoveItem :one
-- Sets exactly one of a location-target (location_id/room_id/spot) or a
-- parent-target (parent_item_id); the service clears the other side before
-- calling this, so the row never carries both a parent and its own place
-- as if it were a root.
UPDATE items
SET location_id    = sqlc.arg(location_id),
    room_id        = sqlc.narg(room_id),
    spot           = sqlc.narg(spot),
    parent_item_id = sqlc.narg(parent_item_id),
    updated_at     = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: SetItemStatus :one
-- The service decides whether the target status is terminal and passes
-- archived_at accordingly; this query just writes whatever it is given.
UPDATE items
SET status      = sqlc.arg(status),
    archived_at = sqlc.narg(archived_at),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveItem :execrows
UPDATE items
SET archived_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND archived_at IS NULL;
