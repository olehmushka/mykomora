-- +goose Up
-- Attention panel snoozes (SPEC 4, "Consumables"; MILESTONES M7).
--
-- Alerts themselves are not stored: they are computed from the item columns
-- on every read, so they can never go stale. All that persists is the
-- family's decision to stop looking at one for a while. A snooze is per
-- family (one member dismissing it hides it for everyone) and per
-- (item, kind): snoozing "below minimum" leaves the same item's "expiring"
-- alert visible.
--
-- `kind` is plain text, like item_events.type, so M9 can add loan_overdue
-- without a migration.
CREATE TABLE alert_dismissals (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id     uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    item_id       uuid        NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    kind          text        NOT NULL,
    snoozed_until date        NOT NULL,
    dismissed_by  uuid        NOT NULL REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alert_dismissals_unique UNIQUE (family_id, item_id, kind)
);

-- +goose Down
DROP TABLE IF EXISTS alert_dismissals;
