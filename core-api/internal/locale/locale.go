// Package locale resolves which of the two supported languages a request
// should be answered in.
//
// SPEC.md's resolution order also includes GeoIP (an IP address geolocated
// against MaxMind's GeoLite2-Country database). That step is deliberately
// dropped here: it would mean an account, a license key and a redistribution/
// update story for a database file, for a single-family app with no public
// sign-up. core-api's own order is therefore just two steps — stored
// preference, then the request's own Accept-Language — before falling back
// to English. The web app resolves a third step (the NEXT_LOCALE cookie)
// before it ever calls core-api, and sends the result as Accept-Language;
// see docs/MILESTONES.md's M2 delivery note.
//
// This split is correct for a future mobile client too, which has no concept
// of that cookie at all.
package locale

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Locale is one of the two languages this application supports.
type Locale string

// The supported locales.
const (
	En Locale = "en"
	Uk Locale = "uk"
)

// Store is the slice of the generated queries this package needs.
type Store interface {
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
}

// Resolver decides the locale a request should be answered in.
type Resolver struct {
	store Store
}

// NewResolver builds the locale resolver.
func NewResolver(store Store) *Resolver {
	return &Resolver{store: store}
}

// Resolve answers in the user's stored preference if they have chosen one,
// otherwise the request's own Accept-Language, otherwise English.
//
// Any failure to load the user is treated the same as "no preference
// chosen": a locale is worth a best-effort guess, never worth failing a
// request over.
func (r *Resolver) Resolve(ctx context.Context, userID pgtype.UUID, acceptLanguage string) Locale {
	if user, err := r.store.GetUserByID(ctx, userID); err == nil && user.Locale.Valid {
		if l, ok := parse(user.Locale.String); ok {
			return l
		}
	}

	if l, ok := ParseAcceptLanguage(acceptLanguage); ok {
		return l
	}

	return En
}

// ParseAcceptLanguage picks the first `en`/`uk` tag the header names, honouring
// its `;q=` weights, highest first and original order preserved on ties.
//
// There is no existing Accept-Language parser in this repo, and
// golang.org/x/text/language would be a large dependency for a two-locale
// system, so this is a small, direct implementation instead.
func ParseAcceptLanguage(header string) (Locale, bool) {
	type weighted struct {
		tag    string
		weight float64
		order  int
	}

	var candidates []weighted

	for i, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		tag, weight := part, 1.0

		if idx := strings.Index(part, ";"); idx >= 0 {
			tag = strings.TrimSpace(part[:idx])

			if q, ok := strings.CutPrefix(strings.TrimSpace(part[idx+1:]), "q="); ok {
				if parsed, err := strconv.ParseFloat(q, 64); err == nil {
					weight = parsed
				}
			}
		}

		// Strip a region subtag ("en-US" -> "en"); only the base language
		// matters to a system with exactly two locales.
		if idx := strings.Index(tag, "-"); idx >= 0 {
			tag = tag[:idx]
		}

		candidates = append(candidates, weighted{tag: strings.ToLower(tag), weight: weight, order: i})
	}

	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].weight > candidates[j].weight })

	for _, c := range candidates {
		if l, ok := parse(c.tag); ok {
			return l, true
		}
	}

	return "", false
}

func parse(tag string) (Locale, bool) {
	switch Locale(tag) {
	case En:
		return En, true
	case Uk:
		return Uk, true
	default:
		return "", false
	}
}
