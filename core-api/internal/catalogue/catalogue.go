// Package catalogue resolves the built-in category and trait labels seeded
// by db/migrations' taxonomy seed into the two supported languages.
//
// It is pure, static data plus a lookup — it holds no family-defined labels
// (those are stored as plain text in name_custom/label_custom and need no
// translation) and makes no database or network call of its own.
package catalogue

import (
	"github.com/olehmushka/mykomora/core-api/internal/locale"
)

// Entry is one catalogue row: a key, matched against name_i18n_key or
// label_i18n_key, and its text in each supported locale.
type Entry struct {
	Key string
	En  string
	Uk  string
}

// Catalogue resolves a key into the text for a given locale.
type Catalogue struct {
	byKey map[string]Entry
}

// New builds a catalogue from a list of entries.
func New(entries []Entry) *Catalogue {
	byKey := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byKey[e.Key] = e
	}

	return &Catalogue{byKey: byKey}
}

// Text resolves a key in the given locale. A missing locale falls back to
// English; a missing key falls back to the bare key itself, so a translator's
// gap never renders as a blank label.
func (c *Catalogue) Text(key string, l locale.Locale) string {
	entry, ok := c.byKey[key]
	if !ok {
		return key
	}

	if l == locale.Uk && entry.Uk != "" {
		return entry.Uk
	}

	if entry.En != "" {
		return entry.En
	}

	return key
}
