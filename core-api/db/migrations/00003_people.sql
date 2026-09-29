-- People (SPEC 3, "Concepts" and SPEC 4, "Identity and tenancy").
--
-- A person is *not* a member. It is anyone the family tracks things **for**: a
-- child with no Google account, a grandparent. The split is deliberate — the
-- second-priority job is kids' sizes, and children will not have accounts.
--
-- `user_id` is the optional bridge back to a member, for an adult who is both.
-- Size history (`person_sizes`) arrives with M8; this table only has to exist
-- for items to point at from M3 onwards.

-- +goose Up
CREATE TABLE people (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id  uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    birthdate  date,
    -- Set when this person is also a member of the family.
    user_id    uuid        REFERENCES users (id) ON DELETE SET NULL,
    is_child   boolean     NOT NULL DEFAULT false,
    avatar_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- Soft delete everywhere (SPEC 7, "Data safety"): archiving a person must
    -- not orphan the items recorded against them.
    archived_at timestamptz
);

CREATE INDEX people_family_live_idx
    ON people (family_id) WHERE archived_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS people;
