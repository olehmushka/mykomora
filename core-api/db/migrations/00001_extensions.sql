-- Postgres extensions the rest of the schema depends on.
--
--   pgcrypto  -- gen_random_uuid() for every primary key
--   unaccent  -- accent-insensitive search (SPEC 6, "Search")
--   pg_trgm   -- fuzzy/substring matching and duplicate hints
--
-- Postgres ships no Ukrainian full-text dictionary, so search is built on the
-- `simple` configuration plus unaccent and trigrams rather than on stemming.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pgcrypto;
