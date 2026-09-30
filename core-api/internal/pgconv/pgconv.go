// Package pgconv converts between the pgx types the generated queries speak
// and the plain Go types the rest of core-api uses.
//
// sqlc emits pgtype values so that SQL NULL survives the round trip. That is
// the right choice at the data layer and the wrong one everywhere else, so the
// translation is collected here rather than repeated in every handler — which
// also keeps the `dupl` linter honest about what is really duplicated.
package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUID wraps an id for a query parameter.
func UUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// UUIDPtr wraps an optional id; nil becomes SQL NULL.
func UUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}

	return UUID(*id)
}

// ToUUID unwraps an id. A NULL column yields the zero UUID, which is never a
// valid key in this schema and so is safe as the "absent" value.
func ToUUID(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.Nil
	}

	return v.Bytes
}

// ToUUIDPtr unwraps a nullable id.
func ToUUIDPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}

	id := uuid.UUID(v.Bytes)

	return &id
}

// Text wraps a string that is always present.
func Text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// TextPtr wraps an optional string; nil becomes SQL NULL.
func TextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}

	return Text(*s)
}

// ToTextPtr unwraps a nullable string.
func ToTextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}

	s := v.String

	return &s
}

// Bool wraps an optional boolean; nil becomes SQL NULL.
func Bool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}

	return pgtype.Bool{Bool: *b, Valid: true}
}

// Timestamptz wraps an instant that is always present.
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ToTime unwraps an instant. A NULL column yields the zero time.
func ToTime(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}

	return v.Time
}

// ToTimePtr unwraps a nullable instant.
func ToTimePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}

	t := v.Time

	return &t
}

// Date wraps an optional calendar date; nil becomes SQL NULL.
func Date(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}

	return pgtype.Date{Time: *t, Valid: true}
}

// ToDatePtr unwraps a nullable calendar date.
func ToDatePtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}

	t := v.Time

	return &t
}
