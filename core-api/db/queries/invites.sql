-- Family invites.
--
-- An owner generates a single-use link; the invitee signs in with Google and
-- joins. There is no email anywhere in the system (SPEC 2, "Notifications"),
-- so the link travels over whatever channel the family already uses.
--
-- Only the SHA-256 of the token is stored. A database dump therefore yields no
-- usable invite links.

-- name: CreateFamilyInvite :one
INSERT INTO family_invites (family_id, token_hash, created_by, expires_at)
VALUES (sqlc.arg(family_id), sqlc.arg(token_hash), sqlc.arg(created_by),
        sqlc.arg(expires_at))
RETURNING id, family_id, created_by, created_at, expires_at,
          accepted_at, accepted_by_user_id, revoked_at;

-- name: GetInviteByTokenHash :one
-- Backs the public `/invites/{token}` preview, so it returns the family's name
-- and nothing else about it: an invite link must be enough to see who invited
-- you, and never enough to see their inventory.
SELECT i.id,
       i.family_id,
       i.expires_at,
       i.accepted_at,
       i.revoked_at,
       f.name AS family_name
FROM family_invites i
         JOIN families f ON f.id = i.family_id
WHERE i.token_hash = $1;

-- name: AcceptInvite :one
-- Single-use, enforced by the statement rather than by the caller: the
-- predicate and the write happen in one round trip, so two invitees racing on
-- the same link cannot both win. No row means expired, revoked or already used.
UPDATE family_invites
SET accepted_at         = now(),
    accepted_by_user_id = $2
WHERE token_hash = $1
  AND accepted_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > now()
RETURNING id, family_id;

-- name: ListFamilyInvites :many
SELECT id, family_id, created_by, created_at, expires_at,
       accepted_at, accepted_by_user_id, revoked_at
FROM family_invites
WHERE family_id = sqlc.arg(family_id)
ORDER BY created_at DESC;

-- name: RevokeFamilyInvite :execrows
UPDATE family_invites
SET revoked_at = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND revoked_at IS NULL
  AND accepted_at IS NULL;
