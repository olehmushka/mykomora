-- Search (SPEC 6, "Search"): the generated tsvector column, its GIN index and
-- the pg_trgm index on `name` that 00008_items.sql deliberately left for this
-- migration.
--
-- Postgres ships no Ukrainian full-text dictionary, so the document is built
-- with the `simple` configuration (no stemming) over unaccented text, and the
-- query side compensates (internal/items/search.go): it matches word
-- prefixes and trims the tail of long words, so "чоботи" finds "чоботах".
--
-- Two things the generated expression has to work around, because generated
-- columns accept only IMMUTABLE expressions:
--
--   * unaccent() is STABLE (it reads a dictionary by name). The wrapper pins
--     the dictionary, which is what makes it safe to declare IMMUTABLE.
--   * An enum cast to text is STABLE too. Season and condition are stored as
--     English enum labels, yet people type «зимові»; the keyword function maps
--     each label to its Ukrainian and English words with a CASE, which only
--     compares enum values and is immutable. It is the one place a new enum
--     label has to be taught to search.
--
-- `traits::text` puts JSON keys in the document as well as values. That is
-- noise ("size_system" is searchable) but harmless, and the only option inside
-- a generated column.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION immutable_unaccent(input text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
    AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, input) $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION item_enum_keywords(item_season item_season, item_condition item_condition) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $$
    SELECT CASE item_season
               WHEN 'summer' THEN 'summer літо літній літні літня літнє'
               WHEN 'winter' THEN 'winter зима зимовий зимові зимова зимове'
               WHEN 'demi' THEN 'demi demiseason демісезон демісезонний демісезонні демісезонна демісезонне'
               WHEN 'all_season' THEN 'all season всесезонний всесезонні цілорічний цілорічні'
               ELSE ''
           END
        || ' '
        || CASE item_condition
               WHEN 'new' THEN 'new нове новий нова нові'
               WHEN 'like_new' THEN 'like new майже нове як нове'
               WHEN 'good' THEN 'good гарний хороший добрий'
               WHEN 'worn' THEN 'worn зношений зношене зношені потертий'
               WHEN 'poor' THEN 'poor поганий погане погані'
               ELSE ''
           END
$$;
-- +goose StatementEnd

ALTER TABLE items ADD COLUMN search_tsv tsvector GENERATED ALWAYS AS (
    to_tsvector(
        'simple',
        immutable_unaccent(
            name
            || ' ' || coalesce(brand, '')
            || ' ' || coalesce(notes, '')
            || ' ' || coalesce(size_value, '')
            || ' ' || coalesce(serial_number, '')
            || ' ' || item_enum_keywords(season, condition)
            || ' ' || traits::text
        )
    )
) STORED;

CREATE INDEX items_search_tsv_idx ON items USING GIN (search_tsv);

-- Fuzzy / substring matching on the name, and duplicate hints. The queries
-- must use this exact expression for the planner to pick the index.
CREATE INDEX items_name_trgm_idx
    ON items USING GIN (immutable_unaccent(lower(name)) gin_trgm_ops);

-- Filter support. Only the filters whose selectivity justifies an index.
CREATE INDEX items_family_status_idx ON items (family_id, status) WHERE archived_at IS NULL;
CREATE INDEX items_family_category_idx ON items (family_id, category_id) WHERE archived_at IS NULL;
CREATE INDEX items_family_person_idx ON items (family_id, for_person_id)
    WHERE archived_at IS NULL AND for_person_id IS NOT NULL;
CREATE INDEX items_family_expires_idx ON items (family_id, expires_at)
    WHERE archived_at IS NULL AND expires_at IS NOT NULL;
CREATE INDEX items_family_warranty_idx ON items (family_id, warranty_until)
    WHERE archived_at IS NULL AND warranty_until IS NOT NULL;
CREATE INDEX item_tags_tag_idx ON item_tags (tag_id);

-- +goose Down
DROP INDEX IF EXISTS item_tags_tag_idx;
DROP INDEX IF EXISTS items_family_warranty_idx;
DROP INDEX IF EXISTS items_family_expires_idx;
DROP INDEX IF EXISTS items_family_person_idx;
DROP INDEX IF EXISTS items_family_category_idx;
DROP INDEX IF EXISTS items_family_status_idx;
DROP INDEX IF EXISTS items_name_trgm_idx;
DROP INDEX IF EXISTS items_search_tsv_idx;
ALTER TABLE items DROP COLUMN IF EXISTS search_tsv;
DROP FUNCTION IF EXISTS item_enum_keywords(item_season, item_condition);
DROP FUNCTION IF EXISTS immutable_unaccent(text);
