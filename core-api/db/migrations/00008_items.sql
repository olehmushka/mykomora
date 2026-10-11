-- Items core (SPEC 3, "Concepts" and SPEC 4, "Items").
--
-- Hybrid columns + traits JSONB, not EAV (SPEC 4, "Why hybrid columns +
-- JSONB, not EAV"): anything filtered across categories is a real column,
-- the long tail and every family-defined custom field live in `traits`.
--
-- Several of these hot columns were already anticipated by M2's taxonomy
-- seed: db/migrations/taxonomy_seed_data.go's `text()` helper placeholder-
-- seeded `size`, `size_system`, `season`, `for_person`, `expires_at`,
-- `opened_at`, `pao_months`, `quantity`, `unit`, `min_quantity`,
-- `serial_number` and `warranty_until` as DataType "text" specifically so
-- this migration could give them a real column and (where the vocabulary is
-- closed) a real data_type, without M2 inventing a type it couldn't use yet.
-- The UPDATE statements below do exactly that. `for_person` and `size` stay
-- `text` placeholders on purpose: `for_person`'s valid values are a family's
-- dynamic `people` list, not a static `enum_options` list, and `size`'s
-- validation against `size_lookup` needs a `size_type` that `trait_defs` has
-- no column for — both are M8's job.
--
-- Containment (`parent_item_id`) is read-resolved, not denormalised: a
-- nested item's own `location_id`/`room_id`/`spot` are set once at creation
-- so the NOT NULL column has something to hold, but become cosmetic the
-- moment it has a parent. Effective place is always the root ancestor's
-- place, computed by internal/items' recursive-CTE queries at read time.
-- Moving a container therefore writes exactly one row.
--
-- `search_tsv` is deliberately absent: the generated tsvector column, its
-- GIN index and the pg_trgm index on `name` are M5's own migration, not
-- this one.

-- +goose Up
CREATE TYPE item_status AS ENUM (
    'in_use', 'stored', 'lent', 'to_give_away',
    'consumed', 'given_away', 'sold', 'discarded', 'lost'
);

-- Vocabularies the spec names as columns but never defines; locked in here.
CREATE TYPE item_condition AS ENUM ('new', 'like_new', 'good', 'worn', 'poor');
CREATE TYPE item_season AS ENUM ('summer', 'winter', 'demi', 'all_season');

CREATE TABLE items (
    id                uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id         uuid          NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    name              text          NOT NULL,
    category_id       uuid          NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,

    -- Place. Authoritative only when parent_item_id IS NULL — see header.
    location_id       uuid          NOT NULL REFERENCES locations (id) ON DELETE RESTRICT,
    room_id           uuid          REFERENCES rooms (id) ON DELETE SET NULL,
    spot              text,

    -- Containment.
    parent_item_id    uuid          REFERENCES items (id) ON DELETE SET NULL,
    is_container      boolean       NOT NULL DEFAULT false,

    status            item_status   NOT NULL DEFAULT 'in_use',

    -- Consumables (SPEC 4, hot columns).
    quantity          numeric       NOT NULL DEFAULT 1,
    unit              text,
    min_quantity      numeric,
    expires_at        date,
    opened_at         date,
    pao_months        integer,

    -- Hot, cross-category columns.
    brand             text,
    condition         item_condition,
    season            item_season,
    size_system       size_system,
    size_value        text,
    -- Populated by M8's fit-matching against size_lookup; NULL until then.
    size_norm_min     numeric,
    size_norm_max     numeric,
    for_person_id     uuid          REFERENCES people (id) ON DELETE SET NULL,
    serial_number     text,
    warranty_until    date,
    purchase_date     date,
    purchase_price    numeric,
    purchase_currency text,
    notes             text,

    -- Long tail + every family-defined custom field.
    traits            jsonb         NOT NULL DEFAULT '{}',

    created_by        uuid          NOT NULL REFERENCES users (id),
    created_at        timestamptz   NOT NULL DEFAULT now(),
    updated_at        timestamptz   NOT NULL DEFAULT now(),
    archived_at       timestamptz,

    CONSTRAINT items_not_own_parent CHECK (parent_item_id <> id)
);

CREATE INDEX items_family_live_idx
    ON items (family_id) WHERE archived_at IS NULL;
CREATE INDEX items_parent_live_idx
    ON items (parent_item_id) WHERE archived_at IS NULL;
-- Root items (no parent) are what location/room browse lists; a nested
-- item's place columns are never queried this way (see header).
CREATE INDEX items_location_room_idx
    ON items (family_id, location_id, room_id)
    WHERE archived_at IS NULL AND parent_item_id IS NULL;
CREATE INDEX items_traits_gin_idx ON items USING GIN (traits);

CREATE TABLE tags (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id uuid NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    name      text NOT NULL,
    color     text
);

-- No archived_at: SPEC's own sketch omits it for this one table (unlike
-- every other), so deleting a tag is a genuine hard delete. item_tags rows
-- referencing it cascade away with it.
CREATE UNIQUE INDEX tags_family_name_idx ON tags (family_id, name);

CREATE TABLE item_tags (
    item_id uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    tag_id  uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);

CREATE TABLE item_events (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id     uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    item_id       uuid        NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    actor_user_id uuid        NOT NULL REFERENCES users (id),
    -- Plain text, not an enum: M7/M8/M9 add event types (quantity_changed,
    -- fits-related, lending) with no migration needed for this column.
    type          text        NOT NULL,
    payload       jsonb       NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX item_events_item_idx ON item_events (item_id, created_at DESC);

-- Reclassify M2's placeholder trait_defs now that real hot columns exist
-- for them (see header). Only non-archived rows: an archived built-in field
-- was retired for a reason unrelated to this migration.
UPDATE trait_defs SET data_type = 'enum',
    enum_options = '["summer", "winter", "demi", "all_season"]'::jsonb
    WHERE key = 'season' AND data_type = 'text' AND archived_at IS NULL;

UPDATE trait_defs SET data_type = 'enum',
    enum_options = '["new", "like_new", "good", "worn", "poor"]'::jsonb
    WHERE key = 'condition' AND data_type = 'text' AND archived_at IS NULL;

UPDATE trait_defs SET data_type = 'enum',
    enum_options = '["kids_cm", "kids_eu", "intl_letter", "eu_adult", "shoe_eu", "shoe_uk", "shoe_us", "one_size"]'::jsonb
    WHERE key = 'size_system' AND data_type = 'text' AND archived_at IS NULL;

UPDATE trait_defs SET data_type = 'integer'
    WHERE key = 'pao_months' AND data_type = 'text' AND archived_at IS NULL;

UPDATE trait_defs SET data_type = 'number'
    WHERE key IN ('quantity', 'min_quantity') AND data_type = 'text' AND archived_at IS NULL;

UPDATE trait_defs SET data_type = 'date'
    WHERE key IN ('expires_at', 'opened_at', 'warranty_until') AND data_type = 'text' AND archived_at IS NULL;

-- +goose Down
UPDATE trait_defs SET data_type = 'text', enum_options = NULL
    WHERE key IN ('season', 'condition', 'size_system') AND data_type = 'enum';
UPDATE trait_defs SET data_type = 'text'
    WHERE key = 'pao_months' AND data_type = 'integer';
UPDATE trait_defs SET data_type = 'text'
    WHERE key IN ('quantity', 'min_quantity') AND data_type = 'number';
UPDATE trait_defs SET data_type = 'text'
    WHERE key IN ('expires_at', 'opened_at', 'warranty_until') AND data_type = 'date';

DROP TABLE IF EXISTS item_events;
DROP TABLE IF EXISTS item_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS items;
DROP TYPE IF EXISTS item_season;
DROP TYPE IF EXISTS item_condition;
DROP TYPE IF EXISTS item_status;
