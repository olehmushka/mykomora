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

-- name: SearchItems :many
-- The one list/search query behind GET /items (SPEC 6, "Search"). Every
-- filter is optional; an absent one matches everything.
--
-- Text: internal/items/search.go turns the user's words into q_or (every
-- token, prefix-matched, OR-ed), q_tokens (the same tokens, one tsquery each)
-- and q_plain. The WHERE below is the index-friendly pre-filter (GIN on
-- search_tsv, trigram GIN on name); the outer filter then demands that at
-- least q_min_match tokens actually hit, or that the name is a close fuzzy
-- match. Requiring "most" rather than "all" tokens is what lets a sentence
-- like «зимові чоботи 32 розміру» find an item that never says "розміру".
--
-- Place: room_id/location_id match the item's EFFECTIVE place, i.e. its root
-- container's, so a boot inside a box is found under the box's room. The
-- `placed` walk only runs when one of them is set.
--
-- Pagination is keyset on (sort_key, id). sort_key is text compared in the C
-- collation and built so that ascending order is always the wanted order
-- (newest-first is encoded by subtracting from a constant).
WITH RECURSIVE cats AS (
    SELECT c.id
    FROM categories c
    WHERE c.id = sqlc.narg(category_id)::uuid
      AND (c.family_id = sqlc.arg(family_id) OR c.family_id IS NULL)
    UNION ALL
    SELECT c.id
    FROM categories c
             JOIN cats ON c.parent_id = cats.id
    WHERE c.family_id = sqlc.arg(family_id) OR c.family_id IS NULL
),
placed AS (
    SELECT r.id
    FROM items r
    WHERE r.family_id = sqlc.arg(family_id)
      AND r.parent_item_id IS NULL
      AND (sqlc.narg(location_id)::uuid IS NULL OR r.location_id = sqlc.narg(location_id))
      AND (sqlc.narg(room_id)::uuid IS NULL OR r.room_id = sqlc.narg(room_id))
    UNION ALL
    SELECT c.id
    FROM items c
             JOIN placed ON c.parent_item_id = placed.id
    WHERE c.family_id = sqlc.arg(family_id)
),
cand AS (
    SELECT i.*,
           CASE WHEN sqlc.narg(q_or)::text IS NULL THEN 0
                ELSE (SELECT count(*)
                      FROM unnest(sqlc.narg(q_tokens)::text[]) AS t
                      WHERE i.search_tsv @@ to_tsquery('simple', immutable_unaccent(t)))
               END AS hits,
           CASE WHEN sqlc.narg(q_or)::text IS NULL THEN 0::float4
                ELSE word_similarity(sqlc.narg(q_plain)::text, immutable_unaccent(lower(i.name)))
               END AS sim
    FROM items i
    WHERE i.family_id = sqlc.arg(family_id)
      AND (sqlc.arg(include_archived)::boolean OR i.archived_at IS NULL)
      AND (NOT sqlc.arg(roots_only)::boolean OR i.parent_item_id IS NULL)
      AND (sqlc.narg(parent_item_id)::uuid IS NULL OR i.parent_item_id = sqlc.narg(parent_item_id))
      AND (sqlc.narg(q_or)::text IS NULL
           OR i.search_tsv @@ to_tsquery('simple', immutable_unaccent(sqlc.narg(q_or)))
           OR sqlc.narg(q_plain)::text <% immutable_unaccent(lower(i.name)))
      AND (sqlc.narg(category_id)::uuid IS NULL OR i.category_id IN (SELECT id FROM cats))
      AND ((sqlc.narg(location_id)::uuid IS NULL AND sqlc.narg(room_id)::uuid IS NULL)
           OR i.id IN (SELECT id FROM placed))
      AND (sqlc.narg(for_person_id)::uuid IS NULL OR i.for_person_id = sqlc.narg(for_person_id))
      AND (sqlc.narg(status)::item_status IS NULL OR i.status = sqlc.narg(status))
      AND (sqlc.narg(season)::item_season IS NULL OR i.season = sqlc.narg(season))
      AND (sqlc.narg(condition)::item_condition IS NULL OR i.condition = sqlc.narg(condition))
      AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
            SELECT 1 FROM item_tags it WHERE it.item_id = i.id AND it.tag_id = sqlc.narg(tag_id)))
      AND (sqlc.narg(trait_filter)::jsonb IS NULL OR i.traits @> sqlc.narg(trait_filter))
      AND (sqlc.narg(expires_from)::date IS NULL OR i.expires_at >= sqlc.narg(expires_from))
      AND (sqlc.narg(expires_to)::date IS NULL OR i.expires_at <= sqlc.narg(expires_to))
      AND (sqlc.narg(warranty_from)::date IS NULL OR i.warranty_until >= sqlc.narg(warranty_from))
      AND (sqlc.narg(warranty_to)::date IS NULL OR i.warranty_until <= sqlc.narg(warranty_to))
      AND (sqlc.narg(price_min)::numeric IS NULL OR i.purchase_price >= sqlc.narg(price_min))
      AND (sqlc.narg(price_max)::numeric IS NULL OR i.purchase_price <= sqlc.narg(price_max))
),
keyed AS (
    SELECT cand.*,
           (CASE sqlc.arg(sort_mode)::text
                WHEN 'relevance' THEN
                    lpad((1000000 - round((cand.hits::float4 / greatest(sqlc.arg(q_token_count)::int, 1) + 0.5 * cand.sim) * 100000))::bigint::text, 8, '0')
                WHEN 'name' THEN
                    immutable_unaccent(lower(cand.name))
                WHEN 'expires' THEN
                    coalesce(to_char(cand.expires_at, 'YYYY-MM-DD'), '9999-12-31')
                WHEN 'price' THEN
                    lpad(coalesce(round(cand.purchase_price * 100)::bigint, 9999999999999999)::text, 16, '0')
                ELSE
                    lpad((4102444800000000 - (extract(epoch FROM cand.created_at) * 1000000)::bigint)::text, 16, '0')
            END)::text COLLATE "C" AS sort_key
    FROM cand
    WHERE sqlc.narg(q_or)::text IS NULL
       OR cand.hits >= sqlc.arg(q_min_match)::int
       OR cand.sim >= 0.6
),
page AS (
    SELECT keyed.id, keyed.sort_key
    FROM keyed
    WHERE sqlc.narg(cursor_key)::text IS NULL
       OR (keyed.sort_key, keyed.id) > (sqlc.narg(cursor_key)::text COLLATE "C", sqlc.narg(cursor_id)::uuid)
    ORDER BY keyed.sort_key, keyed.id
    LIMIT sqlc.arg(result_limit)
)
SELECT sqlc.embed(items), page.sort_key::text AS sort_key, ph.thumb_key AS primary_thumb_key
FROM page
         JOIN items ON items.id = page.id AND items.family_id = sqlc.arg(family_id)
         LEFT JOIN item_photos ph ON ph.item_id = page.id AND ph.is_primary
ORDER BY page.sort_key, page.id;

-- name: ListItemPlacements :many
-- For a page of search hits: each item's ancestor chain with the root's
-- place names, in one round trip. depth 0 is the item itself; the last depth
-- per origin_id is its root, whose location/room are the effective place.
-- Chains are at most maxDepth+1 (5+1) long, so this stays small.
WITH RECURSIVE chain AS (
    SELECT i.id AS origin_id, i.id, i.parent_item_id, i.name, i.location_id, i.room_id, i.spot, 0 AS depth
    FROM items i
    WHERE i.family_id = sqlc.arg(family_id)
      AND i.id = ANY(sqlc.arg(item_ids)::uuid[])
    UNION ALL
    SELECT chain.origin_id, p.id, p.parent_item_id, p.name, p.location_id, p.room_id, p.spot, chain.depth + 1
    FROM items p
             JOIN chain ON p.id = chain.parent_item_id
    WHERE p.family_id = sqlc.arg(family_id)
)
SELECT chain.origin_id,
       chain.id AS ancestor_id,
       chain.name AS ancestor_name,
       chain.location_id,
       chain.room_id,
       chain.spot,
       chain.depth,
       l.name AS location_name,
       r.name AS room_name
FROM chain
         LEFT JOIN locations l ON l.id = chain.location_id AND l.family_id = sqlc.arg(family_id)
         LEFT JOIN rooms r ON r.id = chain.room_id AND r.location_id = l.id
ORDER BY chain.origin_id, chain.depth;

-- name: FindSimilarItems :many
-- Duplicate hints for the create form: close fuzzy matches on the name within
-- the family. Uses the same trigram expression the index is built on.
SELECT i.id, i.name, i.category_id, i.status,
       similarity(immutable_unaccent(lower(i.name)), immutable_unaccent(lower(sqlc.arg(name)::text))) AS score
FROM items i
WHERE i.family_id = sqlc.arg(family_id)
  AND i.archived_at IS NULL
  AND immutable_unaccent(lower(i.name)) % immutable_unaccent(lower(sqlc.arg(name)::text))
  AND similarity(immutable_unaccent(lower(i.name)), immutable_unaccent(lower(sqlc.arg(name)::text))) >= 0.4
ORDER BY score DESC, i.id
LIMIT 5;

-- name: DashboardCountsByLocation :many
-- Live items per location, by effective place: nested items count under the
-- location of their root container.
WITH RECURSIVE tree AS (
    SELECT r.id, r.location_id
    FROM items r
    WHERE r.family_id = sqlc.arg(family_id) AND r.parent_item_id IS NULL AND r.archived_at IS NULL
    UNION ALL
    SELECT c.id, tree.location_id
    FROM items c
             JOIN tree ON c.parent_item_id = tree.id
    WHERE c.family_id = sqlc.arg(family_id) AND c.archived_at IS NULL
)
SELECT l.id AS location_id, l.name AS location_name, count(tree.id) AS item_count
FROM locations l
         LEFT JOIN tree ON tree.location_id = l.id
WHERE l.family_id = sqlc.arg(family_id) AND l.archived_at IS NULL
GROUP BY l.id, l.name, l.sort_order
ORDER BY l.sort_order, l.name;

-- name: DashboardCountsByCategory :many
SELECT i.category_id, count(*) AS item_count
FROM items i
WHERE i.family_id = sqlc.arg(family_id) AND i.archived_at IS NULL
GROUP BY i.category_id
ORDER BY item_count DESC, i.category_id;

-- name: DashboardValueByCurrency :many
-- Purchase value of live items still owned. No conversion between
-- currencies (multi-currency is post-v1): one row per currency.
SELECT coalesce(i.purchase_currency, '') AS currency,
       sum(i.purchase_price * i.quantity)::numeric AS total,
       count(*) AS item_count
FROM items i
WHERE i.family_id = sqlc.arg(family_id)
  AND i.archived_at IS NULL
  AND i.purchase_price IS NOT NULL
  AND i.status IN ('in_use', 'stored', 'lent', 'to_give_away')
GROUP BY 1
ORDER BY 1;

-- name: ListRecentItemEvents :many
SELECT e.*, i.name AS item_name
FROM item_events e
         JOIN items i ON i.id = e.item_id AND i.family_id = sqlc.arg(family_id)
WHERE e.family_id = sqlc.arg(family_id)
ORDER BY e.created_at DESC, e.id
LIMIT sqlc.arg(result_limit);

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

-- name: ExportItems :many
-- One keyset page of the family's live items for CSV export, with every
-- reference resolved to something a person can read (the category as its
-- stable slug, so a re-import finds it again). Ordered by id so the caller can
-- pass the last id back as after_id.
SELECT i.id,
       i.name,
       c.slug AS category_slug,
       l.name AS location_name,
       r.name AS room_name,
       p.name AS container_name,
       i.spot,
       i.is_container,
       i.status,
       i.quantity,
       i.unit,
       i.min_quantity,
       i.expires_at,
       i.opened_at,
       i.pao_months,
       i.brand,
       i.condition,
       i.season,
       i.size_system,
       i.size_value,
       pe.name AS person_name,
       i.serial_number,
       i.warranty_until,
       i.purchase_date,
       i.purchase_price,
       i.purchase_currency,
       i.notes,
       i.traits,
       COALESCE((SELECT string_agg(t.name, ';' ORDER BY t.name)
                 FROM item_tags it
                          JOIN tags t ON t.id = it.tag_id AND t.family_id = sqlc.arg(family_id)
                 WHERE it.item_id = i.id), '')::text AS tag_names
FROM items i
         JOIN categories c ON c.id = i.category_id
         LEFT JOIN locations l ON l.id = i.location_id AND l.family_id = sqlc.arg(family_id)
         LEFT JOIN rooms r ON r.id = i.room_id AND r.location_id = l.id
         LEFT JOIN items p ON p.id = i.parent_item_id AND p.family_id = sqlc.arg(family_id)
         LEFT JOIN people pe ON pe.id = i.for_person_id AND pe.family_id = sqlc.arg(family_id)
WHERE i.family_id = sqlc.arg(family_id)
  AND i.archived_at IS NULL
  AND i.id > sqlc.arg(after_id)
ORDER BY i.id
LIMIT sqlc.arg(page_size);
