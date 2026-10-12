-- Attention panel and shopping list (MILESTONES M7).
--
-- Alerts are computed on read from the item columns; only snoozes are
-- stored. The classification into kinds happens in internal/items so the
-- thresholds live next to their tests — this query only narrows the
-- family's items to the ones that could possibly raise an alert.

-- name: ListAlertCandidates :many
-- effective_expiry is the earlier of the printed expiry and the
-- period-after-opening date (opened_at + pao_months); LEAST ignores NULLs.
SELECT id, name, category_id, quantity, unit, min_quantity,
       expires_at, opened_at, pao_months, warranty_until,
       LEAST(expires_at, (opened_at + make_interval(months => pao_months))::date)::date AS effective_expiry
FROM items
WHERE family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
  AND (
        LEAST(expires_at, (opened_at + make_interval(months => pao_months))::date)
            <= sqlc.arg(expiry_horizon)::date
     OR (min_quantity IS NOT NULL AND quantity < min_quantity)
     OR warranty_until BETWEEN sqlc.arg(today)::date AND sqlc.arg(warranty_horizon)::date
  )
ORDER BY name, id;

-- name: ListActiveAlertDismissals :many
SELECT *
FROM alert_dismissals
WHERE family_id = sqlc.arg(family_id)
  AND snoozed_until > sqlc.arg(today)::date;

-- name: UpsertAlertDismissal :one
-- Joined through items so a dismissal can only ever name this family's item.
INSERT INTO alert_dismissals (family_id, item_id, kind, snoozed_until, dismissed_by)
SELECT sqlc.arg(family_id), i.id, sqlc.arg(kind), sqlc.arg(snoozed_until), sqlc.arg(dismissed_by)
FROM items i
WHERE i.id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id)
  AND i.archived_at IS NULL
ON CONFLICT (family_id, item_id, kind)
DO UPDATE SET snoozed_until = EXCLUDED.snoozed_until,
              dismissed_by  = EXCLUDED.dismissed_by,
              created_at    = now()
RETURNING *;

-- name: DeleteAlertDismissal :execrows
DELETE FROM alert_dismissals
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id);

-- name: ListShoppingItems :many
SELECT id, name, category_id, quantity, unit, min_quantity
FROM items
WHERE family_id = sqlc.arg(family_id)
  AND archived_at IS NULL
  AND min_quantity IS NOT NULL
  AND quantity < min_quantity
ORDER BY name, id;
