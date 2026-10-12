package items

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxQueryTokens = 8
	// minMatchRatio is the share of a query's words an item must contain to
	// count as a hit. Below 1 on purpose: people type sentences
	// («зимові чоботи 32 розміру»), not exact keyword sets.
	minMatchRatio = 0.6
)

// stopwords are words that carry no signal in a possessions search. They are
// dropped from the query, not from the index. "розмір" and "size" are here
// because the size itself is the signal ("32"), and no item says the word.
var stopwords = map[string]struct{}{
	// uk
	"де": {}, "що": {}, "як": {}, "який": {}, "яка": {}, "яке": {}, "які": {}, "мій": {}, "моя": {}, "моє": {},
	"мої": {}, "є": {}, "та": {}, "або": {}, "для": {}, "при": {}, "розмір": {}, "розміру": {}, "розміри": {},
	"розмірі": {}, "розміром": {}, "лежить": {}, "знайти": {}, "знайди": {},
	// en
	"the": {}, "an": {}, "and": {}, "or": {}, "where": {}, "is": {}, "are": {}, "my": {}, "of": {}, "in": {},
	"on": {}, "for": {}, "size": {}, "find": {},
}

// searchQuery is a user's words, prepared for the SearchItems SQL.
type searchQuery struct {
	// Or is a tsquery of every token, prefix-matched, OR-ed together.
	Or string
	// Tokens is the same tokens, one tsquery each, so SQL can count hits.
	Tokens []string
	// Plain is the tokens re-joined, for trigram matching against names.
	Plain string
	// MinMatch is how many of Tokens an item must contain.
	MinMatch int32
}

// empty reports whether there is nothing to search for (no words, or only
// stopwords). The list is then unfiltered rather than empty.
func (q searchQuery) empty() bool { return len(q.Tokens) == 0 }

// parseSearchQuery lowercases the text, splits it into letter/digit words,
// drops stopwords and single letters, and turns each remaining word into a
// prefix lexeme.
//
// Postgres has no Ukrainian stemmer, so the stemming is approximate: a word
// of six or more letters loses its last two, of four or five its last one.
// "чоботи" becomes "чобо:*" and still matches "чоботах". Numbers stay exact:
// "32:*" would also match 320. Output contains only letters and digits, which
// is what makes it safe to hand to to_tsquery.
func parseSearchQuery(text string) searchQuery {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	seen := make(map[string]struct{}, len(words))
	plain := make([]string, 0, len(words))
	tokens := make([]string, 0, len(words))

	for _, word := range words {
		if len(tokens) == maxQueryTokens {
			break
		}

		if _, stop := stopwords[word]; stop {
			continue
		}

		lexeme := lexemeFor(word)
		if lexeme == "" {
			continue
		}

		if _, dup := seen[lexeme]; dup {
			continue
		}

		seen[lexeme] = struct{}{}

		plain = append(plain, word)
		tokens = append(tokens, lexeme)
	}

	if len(tokens) == 0 {
		return searchQuery{}
	}

	return searchQuery{
		Or:       strings.Join(tokens, " | "),
		Tokens:   tokens,
		Plain:    strings.Join(plain, " "),
		MinMatch: minMatch(len(tokens)),
	}
}

func lexemeFor(word string) string {
	n := utf8.RuneCountInString(word)

	if isNumeric(word) {
		return word
	}

	if n < 2 {
		return ""
	}

	keep := n

	switch {
	case n >= 6:
		keep = n - 2
	case n >= 4:
		keep = n - 1
	}

	runes := []rune(word)

	return string(runes[:keep]) + ":*"
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}

	return s != ""
}

// minMatch is ceil(minMatchRatio * n), at least 1.
func minMatch(n int) int32 {
	need := int32(float64(n)*minMatchRatio + 0.999999) //nolint:gosec // n <= maxQueryTokens

	return max(need, 1)
}

// --- keyset cursor ---------------------------------------------------------

// errBadCursor is returned for a cursor that is malformed or was issued for
// a different sort.
var errBadCursor = errors.New("invalid cursor")

type cursor struct {
	Sort string    `json:"s"`
	Key  string    `json:"k"`
	ID   uuid.UUID `json:"i"`
}

func encodeCursor(c cursor) string {
	raw, err := json.Marshal(c)
	if err != nil {
		// Unreachable for two strings and a UUID; an empty cursor is rejected
		// on the way back in, so the worst case is a client restarting the list.
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(token, sort string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return cursor{}, errBadCursor
	}

	var c cursor
	if err := json.Unmarshal(raw, &c); err != nil || c.Sort != sort || c.Key == "" {
		return cursor{}, errBadCursor
	}

	return c, nil
}
