-- Lending (MILESTONES M9). Loans own the `lent` item status; the status
-- change and the item event are written by internal/items in the same
-- transaction as these statements.

-- name: CreateLoan :one
-- Joined through items and (when given) people so a loan can only ever name
-- this family's rows. No row is returned if the item is not the family's.
INSERT INTO loans (family_id, item_id, borrower_person_id, borrower_name, due_at, note, restore_status, created_by)
SELECT sqlc.arg(family_id), i.id, sqlc.narg(borrower_person_id), sqlc.narg(borrower_name),
       sqlc.narg(due_at), sqlc.narg(note), sqlc.arg(restore_status), sqlc.arg(created_by)
FROM items i
WHERE i.id = sqlc.arg(item_id)
  AND i.family_id = sqlc.arg(family_id)
  AND (sqlc.narg(borrower_person_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM people p
        WHERE p.id = sqlc.narg(borrower_person_id) AND p.family_id = sqlc.arg(family_id)))
RETURNING *;

-- name: GetOpenLoanForItem :one
SELECT *
FROM loans
WHERE item_id = sqlc.arg(item_id)
  AND family_id = sqlc.arg(family_id)
  AND returned_at IS NULL;

-- name: ReturnLoan :one
UPDATE loans
SET returned_at = now()
WHERE id = sqlc.arg(id)
  AND family_id = sqlc.arg(family_id)
  AND returned_at IS NULL
RETURNING *;

-- name: ListItemLoans :many
SELECT l.*, p.name AS person_name
FROM loans l
LEFT JOIN people p ON p.id = l.borrower_person_id AND p.family_id = l.family_id
WHERE l.item_id = sqlc.arg(item_id)
  AND l.family_id = sqlc.arg(family_id)
ORDER BY l.lent_at DESC, l.id;

-- name: ListOpenLoans :many
-- Overdue first (oldest due date), then undated and future by due date.
SELECT l.*, p.name AS person_name, i.name AS item_name, i.category_id
FROM loans l
JOIN items i ON i.id = l.item_id AND i.family_id = l.family_id
LEFT JOIN people p ON p.id = l.borrower_person_id AND p.family_id = l.family_id
WHERE l.family_id = sqlc.arg(family_id)
  AND l.returned_at IS NULL
ORDER BY l.due_at NULLS LAST, l.lent_at, l.id;
