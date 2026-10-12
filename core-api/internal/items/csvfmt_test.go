package items

import "testing"

func TestCSVSafeRoundTrips(t *testing.T) {
	for _, in := range []string{"plain", "=SUM(A1)", "+1", "-5", "@x", "'quoted", "'=already", "''-x", ""} {
		if got := csvUnsafe(csvSafe(in)); got != in {
			t.Errorf("csvUnsafe(csvSafe(%q)) = %q", in, got)
		}
	}

	if got := csvSafe("=cmd"); got != "'=cmd" {
		t.Errorf("csvSafe(=cmd) = %q, want a leading apostrophe", got)
	}
}
