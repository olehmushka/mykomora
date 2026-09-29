//go:build integration

// Integration tests for the data layer. They need a Docker daemon, so they sit
// behind the `integration` build tag: `make test` stays fast for the inner loop
// and `make test-integration` (and CI) runs the real thing.
//
// What this proves, and why it exists since M0: the goose migrations apply to a
// real Postgres, and the sqlc-generated code runs against the schema those
// migrations produced. Those are the two halves of the toolchain most likely to
// drift apart silently.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/testdb"
)

// The point of M0: generated query, real schema, real database.
func TestGeneratedQueryRunsAgainstMigratedSchema(t *testing.T) {
	ctx := context.Background()
	database := testdb.Start(t)

	now, err := db.New(database.Pool).GetServerTime(ctx)
	if err != nil {
		t.Fatalf("GetServerTime: %v", err)
	}

	if !now.Valid {
		t.Fatal("GetServerTime returned a NULL timestamp")
	}

	// A clock within a day of ours is enough to prove the value came from
	// Postgres rather than from a zero value.
	if delta := time.Since(now.Time); delta > 24*time.Hour || delta < -24*time.Hour {
		t.Errorf("database clock is %s away from ours; value looks synthetic", delta)
	}
}

// Every extension the later schema depends on must actually be installed, and
// the migration must be reversible — CI checks down/up, and this pins the
// extensions themselves so a silently-dropped CREATE EXTENSION is caught.
func TestMigrationsInstallRequiredExtensions(t *testing.T) {
	ctx := context.Background()
	database := testdb.Start(t)

	for _, name := range []string{"pgcrypto", "unaccent", "pg_trgm"} {
		var installed bool

		const q = `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)`
		if err := database.SQL.QueryRowContext(ctx, q, name).Scan(&installed); err != nil {
			t.Fatalf("query pg_extension for %s: %v", name, err)
		}

		if !installed {
			t.Errorf("extension %s is not installed", name)
		}
	}

	dir := testdb.MigrationsDir()

	if err := goose.Down(database.SQL, dir); err != nil {
		t.Fatalf("goose down: %v", err)
	}

	if err := goose.Up(database.SQL, dir); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
}
