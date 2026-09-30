-- Identity. `users` is the one global table in the schema: a Google account
-- exists before it belongs to any family, so the queries here run *before* a
-- tenant is known and cannot be scoped by one.
--
-- Every query in this file is therefore named in the `family-scoped` vet rule's
-- allowlist in sqlc.yaml. Nothing else in the repo may join that list without
-- the same justification: no tenant row is read or written here.

-- name: UpsertUserByGoogleSub :one
-- TENANCY: pre-tenant. Runs on the OAuth callback, before membership is known.
--
-- Google's `sub` is the stable identity; email and display name are profile
-- data that may change between sign-ins, so they are refreshed every time.
INSERT INTO users (google_sub, email, display_name, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (google_sub) DO UPDATE
SET email        = excluded.email,
    display_name = excluded.display_name,
    avatar_url   = excluded.avatar_url,
    last_seen_at = now()
RETURNING *;

-- name: GetUserByID :one
-- TENANCY: pre-tenant. Resolves the principal carried by an access token; the
-- caller then scopes everything that follows by the token's tenant claim.
SELECT *
FROM users
WHERE id = $1;

-- name: TouchUserLastSeen :exec
-- TENANCY: pre-tenant. Presence is a property of the account, not of a tenant.
UPDATE users
SET last_seen_at = now()
WHERE id = $1;
