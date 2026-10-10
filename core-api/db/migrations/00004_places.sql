-- Places (SPEC 3, "Concepts" and SPEC 4, "Places").
--
-- Location -> room -> free-text spot. There is no deeper hierarchy in the
-- schema: depth below a room is expressed by items.spot and by containers
-- (M3), which keeps this table simple while still letting a box move as a
-- unit.
--
-- `rooms` has no family_id column of its own, same shape as `families`
-- itself: it has an id that would make an obvious-looking tenant column, but
-- the tenant key actually lives one hop away. Every rooms query scopes
-- through locations (SPEC 4, "Tenancy invariant" and CLAUDE.md's note on the
-- two `families` queries) rather than taking a vet-rule exemption.

-- +goose Up
CREATE TYPE location_kind AS ENUM ('home', 'storage', 'garage', 'relative', 'other');

CREATE TABLE locations (
    id         uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id  uuid          NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    name       text          NOT NULL,
    kind       location_kind NOT NULL,
    address    text,
    sort_order integer       NOT NULL DEFAULT 0,
    created_at timestamptz   NOT NULL DEFAULT now(),
    updated_at timestamptz   NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE INDEX locations_family_live_idx
    ON locations (family_id) WHERE archived_at IS NULL;

CREATE TABLE rooms (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id uuid        NOT NULL REFERENCES locations (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    sort_order  integer     NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE INDEX rooms_location_live_idx
    ON rooms (location_id) WHERE archived_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS locations;
DROP TYPE IF EXISTS location_kind;
