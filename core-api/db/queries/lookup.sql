-- Barcode lookup cache. `product_cache` is global public metadata with no
-- tenant column, so neither query takes family_id; both are on the
-- family-scoped allowlist in sqlc.yaml.

-- name: GetProductCache :one
SELECT barcode, source, payload, fetched_at
FROM product_cache
WHERE barcode = $1;

-- name: UpsertProductCache :exec
INSERT INTO product_cache (barcode, source, payload, fetched_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (barcode) DO UPDATE
SET source = EXCLUDED.source, payload = EXCLUDED.payload, fetched_at = now();
