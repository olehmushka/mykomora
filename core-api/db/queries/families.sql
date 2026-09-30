-- Families and membership.
--
-- `families` is the awkward table for the tenancy invariant: its own id *is*
-- the tenant key, so there is no family_id column to filter on. The two
-- queries that read and write a family row therefore scope themselves through
-- family_members, which is stronger than a primary-key lookup — the caller
-- proves membership rather than asserting it — and is what the `family-scoped`
-- vet rule is checking for.

-- name: CreateFamily :one
-- Called once per account that signs in without an existing membership. The
-- returned id is the tenant every later query is scoped by.
INSERT INTO families (name)
VALUES ($1)
RETURNING id AS family_id, name, default_currency, created_at;

-- name: GetFamily :one
-- Scoped through family_members rather than by primary key alone.
--
-- `families` is the one tenant table with no family_id column of its own — its
-- id *is* the tenant key — so "filter on family_id" has to mean something else
-- here. Joining membership is the honest reading: the caller proves they are
-- in the family they are asking about, instead of the query trusting a claim
-- the session made. It is also what lets the `family-scoped` vet rule see a
-- real scope instead of a bare primary-key lookup.
SELECT f.*
FROM families f
         JOIN family_members m ON m.family_id = f.id
WHERE f.id = sqlc.arg(family_id)
  AND m.user_id = sqlc.arg(user_id);

-- name: UpdateFamily :one
-- Partial update: an argument left NULL leaves its column alone, so the caller
-- does not have to read-modify-write the whole row.
--
-- Membership-scoped for the same reason as GetFamily above.
UPDATE families
SET name             = coalesce(sqlc.narg(name), name),
    default_currency = coalesce(sqlc.narg(default_currency), default_currency)
WHERE id = sqlc.arg(family_id)
  AND EXISTS (SELECT 1
              FROM family_members m
              WHERE m.family_id = families.id
                AND m.user_id = sqlc.arg(user_id))
RETURNING id, name, default_currency, created_at;

-- name: AddFamilyMember :one
INSERT INTO family_members (family_id, user_id, role)
VALUES (sqlc.arg(family_id), sqlc.arg(user_id), sqlc.arg(role))
ON CONFLICT (family_id, user_id) DO UPDATE
SET role = family_members.role
RETURNING family_id, user_id, role, joined_at;

-- name: GetFamilyMembership :one
-- The authorisation primitive: does this user belong to this family, and as
-- what. Every family-scoped handler resolves it from the session before it
-- touches anything else.
SELECT family_id, user_id, role, joined_at
FROM family_members
WHERE family_id = sqlc.arg(family_id)
  AND user_id = sqlc.arg(user_id);

-- name: GetFirstFamilyForUser :one
-- One family exists in practice, but the schema is multi-tenant from day one
-- (SPEC 2, "Audience"), so sign-in picks the earliest membership deterministically
-- rather than assuming there is exactly one.
SELECT family_id, role
FROM family_members
WHERE user_id = $1
ORDER BY joined_at, family_id
LIMIT 1;

-- name: ListFamilyMembers :many
SELECT m.family_id,
       m.user_id,
       m.role,
       m.joined_at,
       u.email,
       u.display_name,
       u.avatar_url,
       u.last_seen_at
FROM family_members m
         JOIN users u ON u.id = m.user_id
WHERE m.family_id = sqlc.arg(family_id)
ORDER BY m.joined_at, m.user_id;

-- name: RemoveFamilyMember :execrows
-- Owners are deliberately not removable through this path: demoting or
-- removing the last owner would leave the family unadministrable, and there is
-- no role-change endpoint in M1. Zero affected rows is the caller's signal.
DELETE
FROM family_members
WHERE family_id = sqlc.arg(family_id)
  AND user_id = sqlc.arg(user_id)
  AND role = 'member';
