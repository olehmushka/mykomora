package items

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantTokens []string
		wantMin    int32
	}{
		{"the milestone's own sentence", "де зимові чоботи 32 розміру", []string{"зимо:*", "чобо:*", "32"}, 2},
		{"single short word", "кіт", []string{"кіт:*"}, 1},
		{"numbers stay exact", "320", []string{"320"}, 1},
		{"stopwords only", "де the мій", nil, 0},
		{"punctuation and case", "Зимові,  ЧОБОТИ!", []string{"зимо:*", "чобо:*"}, 2},
		{"duplicates collapse", "чоботи чоботи", []string{"чобо:*"}, 1},
		{"empty", "   ", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSearchQuery(tt.in)
			if tt.wantTokens == nil {
				if !got.empty() {
					t.Fatalf("parseSearchQuery(%q) = %+v, want empty", tt.in, got)
				}

				return
			}

			if !reflect.DeepEqual(got.Tokens, tt.wantTokens) {
				t.Errorf("tokens = %v, want %v", got.Tokens, tt.wantTokens)
			}

			if got.MinMatch != tt.wantMin {
				t.Errorf("MinMatch = %d, want %d", got.MinMatch, tt.wantMin)
			}
		})
	}
}

func TestParseSearchQueryCapsTokens(t *testing.T) {
	got := parseSearchQuery("a1 b2 c3 d4 e5 f6 g7 h8 i9 j10 k11")
	if len(got.Tokens) != maxQueryTokens {
		t.Errorf("got %d tokens, want the cap of %d", len(got.Tokens), maxQueryTokens)
	}
}

// to_tsquery is handed Or verbatim, so it may contain nothing but letters,
// digits and the operators this package writes.
func TestParseSearchQueryEmitsNoTsqueryOperators(t *testing.T) {
	got := parseSearchQuery(`foo' & !bar | (baz) <-> qux:* \ ""`)
	for _, token := range got.Tokens {
		for _, r := range token {
			if r == '&' || r == '!' || r == '|' || r == '(' || r == ')' || r == '<' || r == '>' || r == '\'' || r == '\\' {
				t.Errorf("token %q contains tsquery syntax", token)
			}
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	want := cursor{Sort: sortName, Key: "зимові", ID: uuid.New()}

	got, err := decodeCursor(encodeCursor(want), sortName)
	if err != nil || got != want {
		t.Fatalf("round trip = %+v, %v; want %+v", got, err, want)
	}
}

func TestCursorRejectsOtherSortAndGarbage(t *testing.T) {
	token := encodeCursor(cursor{Sort: sortName, Key: "k", ID: uuid.New()})

	if _, err := decodeCursor(token, sortCreated); err == nil {
		t.Error("a cursor from one sort was accepted by another")
	}

	for _, bad := range []string{"", "!!!", "e30"} {
		if _, err := decodeCursor(bad, sortName); err == nil {
			t.Errorf("decodeCursor(%q) succeeded", bad)
		}
	}
}
