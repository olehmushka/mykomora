package locale_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/locale"
)

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   locale.Locale
		wantOK bool
	}{
		{name: "empty header", header: "", wantOK: false},
		{name: "plain uk", header: "uk", want: locale.Uk, wantOK: true},
		{name: "regional subtag", header: "en-US,en;q=0.9", want: locale.En, wantOK: true},
		{name: "weights decide the winner", header: "en;q=0.5,uk;q=0.9", want: locale.Uk, wantOK: true},
		{name: "unsupported locale only", header: "fr-FR", wantOK: false},
		{name: "unsupported first, supported fallback", header: "fr;q=0.9,uk;q=0.8", want: locale.Uk, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := locale.ParseAcceptLanguage(tt.header)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if ok && got != tt.want {
				t.Fatalf("locale = %q, want %q", got, tt.want)
			}
		})
	}
}

type stubStore struct {
	user db.User
	err  error
}

func (s stubStore) GetUserByID(context.Context, pgtype.UUID) (db.User, error) {
	return s.user, s.err
}

func TestResolverPrefersStoredPreference(t *testing.T) {
	store := stubStore{user: db.User{Locale: pgtype.Text{String: "uk", Valid: true}}}
	r := locale.NewResolver(store)

	got := r.Resolve(context.Background(), pgtype.UUID{Valid: true}, "en")
	if got != locale.Uk {
		t.Fatalf("resolve = %q, want %q", got, locale.Uk)
	}
}

func TestResolverFallsBackToAcceptLanguage(t *testing.T) {
	store := stubStore{err: pgx.ErrNoRows}
	r := locale.NewResolver(store)

	got := r.Resolve(context.Background(), pgtype.UUID{Valid: true}, "uk")
	if got != locale.Uk {
		t.Fatalf("resolve = %q, want %q", got, locale.Uk)
	}
}

func TestResolverFallsBackToEnglish(t *testing.T) {
	store := stubStore{err: errors.New("boom")}
	r := locale.NewResolver(store)

	got := r.Resolve(context.Background(), pgtype.UUID{Valid: true}, "")
	if got != locale.En {
		t.Fatalf("resolve = %q, want %q", got, locale.En)
	}
}
