-- Refresh tokens (SPEC 6, "core-api owns identity").
--
-- Rotating refresh tokens with reuse detection: every refresh issues a new
-- token and retires the old one. A token that is presented after it has been
-- rotated away can only have been captured, so it triggers a revocation of the
-- whole account rather than a quiet failure.
--
-- Tokens are stored as SHA-256 digests. The raw value exists only in the
-- client's cookie.

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, user_agent, ip)
VALUES (sqlc.arg(user_id), sqlc.arg(family_id), sqlc.arg(token_hash),
        sqlc.arg(expires_at), sqlc.narg(user_agent), sqlc.narg(ip))
RETURNING id, user_id, family_id, issued_at, expires_at;

-- name: GetRefreshTokenByHash :one
-- `rotated_to` is what makes reuse detectable: a non-NULL value means this
-- token was already exchanged, and the presenter should not still have it.
SELECT id, user_id, family_id, issued_at, expires_at, rotated_to, revoked_at
FROM refresh_tokens
WHERE token_hash = $1;

-- name: RotateRefreshToken :execrows
-- Retires a token in favour of its successor. Both columns are set: the
-- successor makes reuse detectable, and the revocation keeps the live-token
-- index honest.
UPDATE refresh_tokens
SET rotated_to = sqlc.arg(rotated_to),
    revoked_at = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND revoked_at IS NULL;

-- name: RevokeRefreshToken :execrows
-- Sign-out. Scoped to the session's family so a token can only ever be
-- revoked by the tenant it was issued for.
UPDATE refresh_tokens
SET revoked_at = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :execrows
-- TENANCY: deliberately account-wide, and the only write allowlisted in
-- sqlc.yaml's `family-scoped` rule.
--
-- This runs only on reuse detection. A captured token is evidence that the
-- account is compromised, not that one tenant is — so the revocation crosses
-- every family the user belongs to, on purpose. Scoping it would leave the
-- attacker holding live sessions elsewhere.
UPDATE refresh_tokens
SET revoked_at = now()
WHERE user_id = $1
  AND revoked_at IS NULL;
