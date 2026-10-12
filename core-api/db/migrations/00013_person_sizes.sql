-- Kids' sizes and seasons (M8; SPEC 4, "Size normalisation" and SPEC 5).
--
-- Three things land here:
--
--  * `person_sizes`: a person's size history. The newest row per size_type is
--    the current size; older rows are kept so growth is visible.
--  * Size normalisation as a database rule. `size_norm_range` resolves a
--    (system, type, value) label against `size_lookup`, and triggers apply it
--    on `items` and `person_sizes`. Every write path (create, update, batch
--    add, CSV import) therefore normalises identically with no per-path code
--    to forget. Unmapped labels leave the range NULL: they store and display
--    normally but do not take part in fit matching (SPEC 4).
--  * `item_events.batch_id`: groups the events of one bulk action (a seasonal
--    pack/unpack, an "outgrown" sweep) so the whole action can be undone at once.
--
-- Label handling, beyond an exact lookup hit:
--   * ranges such as "104-110" (hyphen, en or em dash) resolve to the span from
--     the first label's minimum to the last label's maximum;
--   * for kids_eu, an age suffix ("4T", "5y", "6 yrs") is ignored.
-- A range resolves only if every part resolves.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION size_norm_range(p_system size_system, p_type size_type, p_value text)
RETURNS TABLE (norm_min numeric, norm_max numeric)
LANGUAGE sql STABLE AS $$
    WITH parts AS (
        SELECT btrim(part) AS label
        FROM unnest(regexp_split_to_array(btrim(p_value), '\s*[-–—]\s*')) AS part
    )
    SELECT min(l.norm_min), max(l.norm_max)
    FROM parts p
    JOIN size_lookup l
      ON l.system = p_system
     AND l.size_type = p_type
     AND (l.value = p.label
          OR (p_system = 'kids_eu'
              AND l.value = regexp_replace(p.label, '\s*(t|y|yrs?)$', '', 'i')))
    HAVING count(*) = (SELECT count(*) FROM parts)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION items_normalise_size() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.size_norm_min := NULL;
    NEW.size_norm_max := NULL;

    IF NEW.size_system IS NOT NULL AND NEW.size_value IS NOT NULL THEN
        -- An item does not say whether it is clothing or footwear; the
        -- system does. one_size and the letter/number systems are clothing.
        SELECT r.norm_min, r.norm_max INTO NEW.size_norm_min, NEW.size_norm_max
        FROM size_norm_range(
            NEW.size_system,
            CASE WHEN NEW.size_system::text LIKE 'shoe\_%' THEN 'shoe'::size_type ELSE 'clothing'::size_type END,
            NEW.size_value) r;
    END IF;

    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER items_normalise_size
    BEFORE INSERT OR UPDATE OF size_system, size_value ON items
    FOR EACH ROW EXECUTE FUNCTION items_normalise_size();

-- Existing items: re-assigning a trigger column fires the trigger.
UPDATE items SET size_system = size_system WHERE size_system IS NOT NULL AND size_value IS NOT NULL;

CREATE TABLE person_sizes (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id     uuid        NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    size_type     size_type   NOT NULL,
    size_system   size_system NOT NULL,
    size_value    text        NOT NULL,
    size_norm_min numeric,
    size_norm_max numeric,
    recorded_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT person_sizes_range_order CHECK (size_norm_min <= size_norm_max)
);

CREATE INDEX person_sizes_person_idx ON person_sizes (person_id, size_type, recorded_at DESC);

-- +goose StatementBegin
CREATE FUNCTION person_sizes_normalise() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    SELECT r.norm_min, r.norm_max INTO NEW.size_norm_min, NEW.size_norm_max
    FROM size_norm_range(NEW.size_system, NEW.size_type, NEW.size_value) r;

    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER person_sizes_normalise
    BEFORE INSERT OR UPDATE OF size_system, size_type, size_value ON person_sizes
    FOR EACH ROW EXECUTE FUNCTION person_sizes_normalise();

ALTER TABLE item_events ADD COLUMN batch_id uuid;
CREATE INDEX item_events_batch_idx ON item_events (family_id, batch_id) WHERE batch_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS item_events_batch_idx;
ALTER TABLE item_events DROP COLUMN IF EXISTS batch_id;
DROP TABLE IF EXISTS person_sizes;
DROP FUNCTION IF EXISTS person_sizes_normalise();
DROP TRIGGER IF EXISTS items_normalise_size ON items;
DROP FUNCTION IF EXISTS items_normalise_size();
DROP FUNCTION IF EXISTS size_norm_range(size_system, size_type, text);
