package apimap

import (
	"testing"
	"time"
)

func TestEffectiveExpiry(t *testing.T) {
	d := func(m time.Month, day int) *time.Time {
		v := time.Date(2026, m, day, 0, 0, 0, 0, time.UTC)

		return &v
	}
	months := func(n int32) *int32 { return &n }

	cases := []struct {
		name            string
		expires, opened *time.Time
		pao             *int32
		want            *time.Time
	}{
		{"neither", nil, nil, nil, nil},
		{"printed expiry only", d(time.December, 1), nil, nil, d(time.December, 1)},
		{"pao without opened date is ignored", nil, nil, months(6), nil},
		{"opened without pao is ignored", nil, d(time.January, 1), nil, nil},
		{"pao only", nil, d(time.January, 15), months(6), d(time.July, 15)},
		{"pao earlier than printed", d(time.December, 1), d(time.January, 15), months(6), d(time.July, 15)},
		{"pao clamps to month end like Postgres", nil, d(time.January, 31), months(1), d(time.February, 28)},
		{"printed earlier than pao", d(time.March, 1), d(time.January, 15), months(6), d(time.March, 1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveExpiry(tc.expires, tc.opened, tc.pao)

			switch {
			case got == nil && tc.want == nil:
			case got == nil || tc.want == nil || !got.Equal(*tc.want):
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
