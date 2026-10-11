// Package pgconv converts between the pgx types the generated queries speak
// and the plain Go types the rest of core-api uses.
//
// sqlc emits pgtype values so that SQL NULL survives the round trip. That is
// the right choice at the data layer and the wrong one everywhere else, so the
// translation is collected here rather than repeated in every handler — which
// also keeps the `dupl` linter honest about what is really duplicated.
package pgconv

import (
	"encoding/json"
	"strconv"
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

// Int4 wraps a present int32 for a nullable (sqlc.narg) query parameter.
func Int4(v int32) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: true}
}

// Int4Ptr wraps an optional int32; nil becomes SQL NULL.
func Int4Ptr(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}

	return Int4(*v)
}

// ToInt4Ptr unwraps a nullable int32.
func ToInt4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}

	value := v.Int32

	return &value
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

// Numeric wraps a float that is always present. pgtype.Numeric only scans a
// string, not a float directly, so the value is formatted first; a finite
// float64 formatted with FormatFloat always parses back, so a scan failure
// here can only mean a non-finite input, which no caller passes.
func Numeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}

	return n
}

// NumericPtr wraps an optional float; nil becomes SQL NULL.
func NumericPtr(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}

	return Numeric(*f)
}

// ToNumeric unwraps a float that is always present. A NULL column yields 0.
func ToNumeric(v pgtype.Numeric) float64 {
	if !v.Valid {
		return 0
	}

	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}

	return f.Float64
}

// ToNumericPtr unwraps a nullable float.
func ToNumericPtr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}

	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}

	value := f.Float64

	return &value
}

// JSONB marshals a plain map for storage; nil becomes an empty object so a
// column declared NOT NULL never gets a Go nil.
func JSONB(m map[string]any) []byte {
	if m == nil {
		m = map[string]any{}
	}

	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}

	return b
}

// ToJSONB unmarshals a stored traits blob. A malformed or empty column reads
// as empty rather than erroring, the same trade taxonomy's decodeEnumOptions
// makes for a column nothing in this package writes maliciously.
func ToJSONB(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}

	return m
}
