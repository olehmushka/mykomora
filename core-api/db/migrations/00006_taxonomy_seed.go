// Package migrations holds the Go-function goose migrations.
//
// This is the first one in the repo (SPEC 4, "Taxonomy": the 23 built-in
// categories and their trait schema are seeded by a goose Go migration, not
// plain SQL, so the data and its loop stay in one place instead of 23
// near-identical statements). That has a real consequence for how migrations
// run: goose.AddMigrationContext only registers a migration with the running
// process's *own* registry, so the stock `goose` CLI — which never imports
// application code — cannot see it. core-api/cmd/migrate exists because of
// this file: it blank-imports this package so goose's library API (which
// internal/testdb already used, for the same reason) has something to find.
//
// See taxonomy_seed_data.go for the seed content itself.
package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(up00006TaxonomySeed, down00006TaxonomySeed)
}

// up00006TaxonomySeed inserts every built-in category and trait definition.
//
// ON CONFLICT ... DO NOTHING against the partial unique indexes 00005
// defined (categories_builtin_slug_idx, trait_defs_builtin_key_idx) makes
// this safe to run twice, which `goose up`/`down`/`up` in CI exercises.
func up00006TaxonomySeed(ctx context.Context, tx *sql.Tx) error {
	for _, cat := range BuiltinCategories {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (slug, icon, name_i18n_key, sort_order)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (slug) WHERE family_id IS NULL DO NOTHING`,
			cat.Slug, cat.Icon, cat.NameKey, cat.SortOrder,
		); err != nil {
			return fmt.Errorf("seed category %s: %w", cat.Slug, err)
		}

		for _, tr := range cat.Traits {
			var enumOptions []byte
			if tr.EnumOptions != nil {
				var err error

				enumOptions, err = json.Marshal(tr.EnumOptions)
				if err != nil {
					return fmt.Errorf("marshal enum options for %s.%s: %w", cat.Slug, tr.Key, err)
				}
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO trait_defs
					(category_id, key, data_type, unit, enum_options, label_i18n_key, required, filterable, sort_order)
				SELECT c.id, $2, $3, $4, $5, $6, $7, $8, $9
				FROM categories c WHERE c.slug = $1 AND c.family_id IS NULL
				ON CONFLICT (category_id, key) WHERE family_id IS NULL DO NOTHING`,
				cat.Slug, tr.Key, tr.DataType, tr.Unit, nullableJSON(enumOptions), tr.LabelKey,
				tr.Required, tr.Filterable, tr.SortOrder,
			); err != nil {
				return fmt.Errorf("seed trait %s.%s: %w", cat.Slug, tr.Key, err)
			}
		}
	}

	return nil
}

// down00006TaxonomySeed removes every built-in row; trait_defs cascades from
// categories.
func down00006TaxonomySeed(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE family_id IS NULL`); err != nil {
		return fmt.Errorf("remove built-in categories: %w", err)
	}

	return nil
}

// nullableJSON turns an absent marshalled value into SQL NULL rather than the
// four bytes `null`, so a non-enum trait's enum_options column stays NULL —
// which is what the enum-shape CHECK constraint on trait_defs expects.
func nullableJSON(b []byte) any {
	if b == nil {
		return nil
	}

	return b
}
