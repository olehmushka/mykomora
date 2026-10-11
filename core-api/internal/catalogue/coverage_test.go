package catalogue_test

import (
	"testing"

	"github.com/olehmushka/mykomora/core-api/db/migrations"
	"github.com/olehmushka/mykomora/core-api/internal/catalogue"
)

// TestEverySeedKeyHasALabel guards against the migration and the catalogue
// drifting apart: every NameKey/LabelKey the seed migration references must
// resolve to something, and every catalogue entry must actually be used by
// the seed, or it is dead translation nobody can reach.
func TestEverySeedKeyHasALabel(t *testing.T) {
	referenced := map[string]struct{}{}

	for _, cat := range migrations.BuiltinCategories {
		referenced[cat.NameKey] = struct{}{}

		for _, tr := range cat.Traits {
			referenced[tr.LabelKey] = struct{}{}
		}
	}

	defined := map[string]struct{}{}
	for _, e := range catalogue.BuiltinLabels {
		defined[e.Key] = struct{}{}
	}

	for key := range referenced {
		if _, ok := defined[key]; !ok {
			t.Errorf("seed migration references %q, which has no catalogue entry", key)
		}
	}

	for key := range defined {
		if _, ok := referenced[key]; !ok {
			t.Errorf("catalogue defines %q, which no seed category or trait references", key)
		}
	}
}

// TestNoEntryIsBlank catches a copy-paste gap: a present-but-empty label
// would otherwise render as nothing, the one outcome Catalogue.Text is meant
// to make impossible.
func TestNoEntryIsBlank(t *testing.T) {
	for _, e := range catalogue.BuiltinLabels {
		if e.En == "" {
			t.Errorf("entry %q has no English label", e.Key)
		}

		if e.Uk == "" {
			t.Errorf("entry %q has no Ukrainian label", e.Key)
		}
	}
}
