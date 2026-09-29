//go:build integration

// Integration tests for the data layer. They need a Docker daemon, so they sit
// behind the `integration` build tag: `make test` stays fast for the inner loop
// and `make test-integration` (and CI) runs the real thing.
//
// What this proves, and why it exists in M0: the goose migrations apply to a
// real Postgres, and the sqlc-generated code runs against the schema those
// migrations produced. Those are the two halves of the toolchain most likely to
// drift apart silently.
package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

const migrationsDir = "../../db/migrations"

// startPostgres boots a throwaway Postgres and returns its connection string.
func startPostgres(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("mykomora_test"),
		tcpostgres.WithUsername("mykomora"),
		tcpostgres.WithPassword("mykomora"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	return dsn
}

// migrate applies every goose migration, and returns the sql.DB so a caller can
// also exercise the down path.
func migrate(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Logf("close database: %v", err)
		}
	})

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	return sqlDB
}

// The point of M0: generated query, real schema, real database.
func TestGeneratedQueryRunsAgainstMigratedSchema(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(t)
	migrate(t, dsn)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	now, err := db.New(pool).GetServerTime(ctx)
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
	dsn := startPostgres(t)
	sqlDB := migrate(t, dsn)

	for _, name := range []string{"pgcrypto", "unaccent", "pg_trgm"} {
		var installed bool

		const q = `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)`
		if err := sqlDB.QueryRowContext(ctx, q, name).Scan(&installed); err != nil {
			t.Fatalf("query pg_extension for %s: %v", name, err)
		}

		if !installed {
			t.Errorf("extension %s is not installed", name)
		}
	}

	if err := goose.Down(sqlDB, migrationsDir); err != nil {
		t.Fatalf("goose down: %v", err)
	}

	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
}
