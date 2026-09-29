-- Operational queries. No tenant data lives here.

-- name: GetServerTime :one
-- Round-trip probe: proves the generated code, the pool and Postgres all agree.
--
-- TENANCY: deliberately not family-scoped, and the only such query in the repo.
-- It is allowlisted by name in sqlc.yaml's `family-scoped` vet rule, so adding
-- another unscoped query requires an obvious, reviewable change to that list.
SELECT now()::timestamptz AS now;
