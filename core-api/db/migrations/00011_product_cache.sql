-- +goose Up
-- Barcode/ISBN lookup cache (SPEC 5, "Barcode / ISBN"). The answers come from
-- public catalogues (Open Food/Beauty/Products Facts, Google Books) and say
-- nothing about any family, so this is a global table with no family_id — the
-- two queries on it are on the family-scoped allowlist in sqlc.yaml for that
-- reason.
--
-- `source` names the catalogue that answered, or 'none' for a remembered
-- miss; the payload is the normalised product the API returns. Misses are
-- cached too, for less time, so rescanning an unknown code stays instant.
CREATE TABLE product_cache (
    barcode    text        PRIMARY KEY,
    source     text        NOT NULL,
    payload    jsonb       NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS product_cache;
