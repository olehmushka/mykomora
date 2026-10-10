-- Taxonomy: categories and their trait schema (SPEC 3, "Concepts" and
-- SPEC 4, "Taxonomy").
--
-- `family_id NULL` means built-in: the label is resolved from the Go message
-- catalogue via `name_i18n_key`/`label_i18n_key`. `family_id` set means
-- family-defined: the label is the family's own text in `name_custom`/
-- `label_custom`. One pair of tables, one code path, one validator, one
-- renderer for both cases (SPEC 4, "Taxonomy").
--
-- Built-in rows are seeded by the next migration, 00006, which is the first
-- Go-function goose migration in this repo (see its own header for why that
-- changes how migrations are run).
--
-- `family_id` is a real, nullable column on both tables, so the sqlc
-- family-scoped vet rule is satisfied by the literal text in every query
-- without needing an allowlist entry: reads are
-- `WHERE family_id = $1 OR family_id IS NULL`, writes are plain
-- `WHERE family_id = $1`, which can never touch a built-in (NULL) row.

-- +goose Up
CREATE TYPE trait_data_type AS ENUM ('text', 'number', 'integer', 'bool', 'date', 'enum', 'size');

CREATE TABLE categories (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id     uuid        REFERENCES families (id) ON DELETE CASCADE,
    parent_id     uuid        REFERENCES categories (id) ON DELETE CASCADE,
    slug          text        NOT NULL,
    icon          text,
    name_i18n_key text,
    name_custom   text,
    sort_order    integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    archived_at   timestamptz,
    CONSTRAINT categories_label_source CHECK (
        (family_id IS NULL AND name_i18n_key IS NOT NULL AND name_custom IS NULL)
        OR
        (family_id IS NOT NULL AND name_custom IS NOT NULL AND name_i18n_key IS NULL)
    )
);

-- A built-in slug is unique across the whole catalogue; a family-defined slug
-- only has to be unique within that family, so two families can both call a
-- category "garage-tools" without colliding.
CREATE UNIQUE INDEX categories_builtin_slug_idx
    ON categories (slug) WHERE family_id IS NULL;
CREATE UNIQUE INDEX categories_family_slug_idx
    ON categories (family_id, slug) WHERE family_id IS NOT NULL;
CREATE INDEX categories_parent_idx ON categories (parent_id);
CREATE INDEX categories_family_live_idx
    ON categories (family_id) WHERE archived_at IS NULL;

CREATE TABLE trait_defs (
    id             uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id    uuid            NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    family_id      uuid            REFERENCES families (id) ON DELETE CASCADE,
    key            text            NOT NULL,
    data_type      trait_data_type NOT NULL,
    unit           text,
    enum_options   jsonb,
    label_i18n_key text,
    label_custom   text,
    required       boolean         NOT NULL DEFAULT false,
    filterable     boolean         NOT NULL DEFAULT false,
    sort_order     integer         NOT NULL DEFAULT 0,
    created_at     timestamptz     NOT NULL DEFAULT now(),
    updated_at     timestamptz     NOT NULL DEFAULT now(),
    archived_at    timestamptz,
    CONSTRAINT trait_defs_label_source CHECK (
        (family_id IS NULL AND label_i18n_key IS NOT NULL AND label_custom IS NULL)
        OR
        (family_id IS NOT NULL AND label_custom IS NOT NULL AND label_i18n_key IS NULL)
    ),
    CONSTRAINT trait_defs_enum_shape CHECK (
        (data_type = 'enum' AND enum_options IS NOT NULL) OR (data_type <> 'enum')
    )
);

CREATE UNIQUE INDEX trait_defs_builtin_key_idx
    ON trait_defs (category_id, key) WHERE family_id IS NULL;
CREATE UNIQUE INDEX trait_defs_family_key_idx
    ON trait_defs (category_id, family_id, key) WHERE family_id IS NOT NULL;
CREATE INDEX trait_defs_category_live_idx
    ON trait_defs (category_id) WHERE archived_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS trait_defs;
DROP TABLE IF EXISTS categories;
DROP TYPE IF EXISTS trait_data_type;
