package items

import "testing"

func TestFitRange(t *testing.T) {
	lo, hi := 107.0, 113.0

	tests := []struct {
		name         string
		min, max     *float64
		offset       int
		wantLo, want float64
		wantOK       bool
	}{
		{"current", &lo, &hi, 0, 107, 113, true},
		{"next size up is the same width above", &lo, &hi, 1, 113, 119, true},
		{"unmapped size has no range", nil, nil, 0, 0, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLo, gotHi, ok := fitRange(tc.min, tc.max, tc.offset)
			if ok != tc.wantOK || gotLo != tc.wantLo || gotHi != tc.want {
				t.Errorf("fitRange = (%v, %v, %v), want (%v, %v, %v)", gotLo, gotHi, ok, tc.wantLo, tc.want, tc.wantOK)
			}
		})
	}
}
