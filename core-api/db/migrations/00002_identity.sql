-- Identity and tenancy (SPEC 4, "Identity and tenancy").
--
-- `users` is the only global table in the schema: a Google account exists
-- before it belongs to anything, and the same account may in principle be a
-- member of several families. Everything else here is tenant data and carries
-- a `family_id`, which is what the sqlc `family-scoped` vet rule enforces.
--
-- Secrets are stored hashed, never raw: invite tokens and refresh tokens are
-- SHA-256 digests, so a database dump cannot be replayed against the API.

-- +goose Up
CREATE TYPE family_role AS ENUM ('owner', 'member');

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub   text        NOT NULL UNIQUE,
    email        text        NOT NULL,
    display_name text        NOT NULL,
    avatar_url   text,
    -- NULL means "not chosen": the locale is then resolved per request (M2).
    locale       text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE families (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text        NOT NULL,
    default_currency text        NOT NULL DEFAULT 'UAH',
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE family_members (
    family_id uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    user_id   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role      family_role NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (family_id, user_id)
);

-- "Which families does this user belong to" runs on every single request, in
-- the opposite direction to the primary key.
CREATE INDEX family_members_user_id_idx ON family_members (user_id);

CREATE TABLE family_invites (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id           uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    -- SHA-256 of the token in the link. The raw token exists only in the URL
    -- the owner copies; it is never written down here.
    token_hash          bytea       NOT NULL UNIQUE,
    created_by          uuid        NOT NULL REFERENCES users (id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    accepted_at         timestamptz,
    accepted_by_user_id uuid        REFERENCES users (id),
    revoked_at          timestamptz,
    CONSTRAINT family_invites_accepted_consistent
        CHECK ((accepted_at IS NULL) = (accepted_by_user_id IS NULL))
);

CREATE INDEX family_invites_family_id_idx ON family_invites (family_id);

CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id  uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL UNIQUE,
    issued_at  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    -- Set when this token is rotated away. Presenting a token that already has
    -- a successor is the reuse signal (SPEC 6, "core-api owns identity").
    rotated_to uuid        REFERENCES refresh_tokens (id) ON DELETE SET NULL,
    revoked_at timestamptz,
    user_agent text,
    ip         inet
);

-- Reuse detection revokes every live token of the user in one statement.
CREATE INDEX refresh_tokens_live_user_idx
    ON refresh_tokens (user_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS family_invites;
DROP TABLE IF EXISTS family_members;
DROP TABLE IF EXISTS families;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS family_role;
