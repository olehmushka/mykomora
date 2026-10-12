-- Lending (SPEC 4, "Movement and accountability"; MILESTONES M9).
--
-- A loan is one item going to one borrower: a family `people` row or a
-- free-text outsider, never both. At most one loan per item is open
-- (returned_at IS NULL), enforced by a partial unique index so two racing
-- lend requests cannot both win.
--
-- `restore_status` remembers the status the item had before it became
-- `lent`, so returning it puts it back where it was. Loans own the `lent`
-- status: internal/items sets it on lend and clears it on return.
--
-- The SPEC table has no family_id; it is added here so every loan query can
-- be family-scoped directly.

-- +goose Up
CREATE TABLE loans (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id            uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    item_id              uuid        NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    borrower_person_id   uuid        REFERENCES people (id),
    borrower_name        text,
    lent_at              timestamptz NOT NULL DEFAULT now(),
    due_at               date,
    returned_at          timestamptz,
    note                 text,
    restore_status       item_status NOT NULL,
    created_by           uuid        NOT NULL REFERENCES users (id),
    CONSTRAINT loans_one_borrower CHECK (
        (borrower_person_id IS NOT NULL) <> (borrower_name IS NOT NULL)
    ),
    CONSTRAINT loans_restore_active CHECK (restore_status IN ('in_use', 'stored', 'to_give_away'))
);

CREATE UNIQUE INDEX loans_one_open_per_item ON loans (item_id) WHERE returned_at IS NULL;
CREATE INDEX loans_open_by_family ON loans (family_id, due_at) WHERE returned_at IS NULL;
CREATE INDEX loans_by_item ON loans (item_id, lent_at DESC);

-- +goose Down
DROP TABLE IF EXISTS loans;
