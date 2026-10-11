-- Photos and attachments (SPEC 4, "Items"; SPEC 6, "Photos and attachments").
--
-- Neither table carries a family_id column — like rooms (db/queries/rooms.sql)
-- and families themselves, their tenant key lives one hop away, through
-- item_id. Every query against them joins through items rather than taking a
-- family-scoped vet-rule exemption (CLAUDE.md, "Every query is family-scoped").
--
-- storage_key/thumb_key point into the object store, not into this database;
-- core-api never stores a URL, only the key a presigned GET is minted from at
-- read time. Large files never pass through Go (SPEC 6): the client resizes,
-- requests a presigned PUT per object, uploads directly, then confirms here
-- with the key plus whatever metadata the object itself can't tell us without
-- a HEAD request (width/height — bytes and content-type the confirm step
-- re-verifies against the real object).

-- +goose Up
CREATE TYPE attachment_kind AS ENUM ('receipt', 'manual', 'warranty', 'other');

CREATE TABLE item_photos (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id      uuid        NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    storage_key  text        NOT NULL,
    thumb_key    text        NOT NULL,
    width        integer     NOT NULL,
    height       integer     NOT NULL,
    bytes        bigint      NOT NULL,
    content_type text        NOT NULL,
    is_primary   boolean     NOT NULL DEFAULT false,
    sort_order   integer     NOT NULL DEFAULT 0,
    uploaded_by  uuid        NOT NULL REFERENCES users (id),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX item_photos_item_idx ON item_photos (item_id, sort_order, id);

-- At most one primary photo per item: a partial unique index on item_id,
-- restricted to the rows where is_primary is true.
CREATE UNIQUE INDEX item_photos_primary_idx ON item_photos (item_id) WHERE is_primary;

CREATE TABLE item_attachments (
    id           uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id      uuid            NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    kind         attachment_kind NOT NULL,
    storage_key  text            NOT NULL,
    filename     text            NOT NULL,
    content_type text            NOT NULL,
    bytes        bigint          NOT NULL,
    uploaded_by  uuid            NOT NULL REFERENCES users (id),
    created_at   timestamptz     NOT NULL DEFAULT now()
);

CREATE INDEX item_attachments_item_idx ON item_attachments (item_id, created_at, id);

-- +goose Down
DROP TABLE IF EXISTS item_attachments;
DROP TABLE IF EXISTS item_photos;
DROP TYPE IF EXISTS attachment_kind;
