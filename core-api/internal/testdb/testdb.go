//go:build integration

// Package testdb boots a throwaway Postgres for integration tests.
//
// It exists so that the container plumbing is written once: M0 had a single
// integration test and could keep its helpers local, but M1 exercises the same
// real schema from two packages, and a second copy of this would be the first
// place the two drift apart.
//
// Behind the `integration` build tag, like its callers: `make test` stays a
// fast inner loop with no Docker, and `make test-integration` runs the real
// thing.
package testdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, for goose
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// DB is a migrated, throwaway database and the two handles onto it: the pgx
// pool the application uses, and the database/sql handle goose needs.
type DB struct {
	Pool *pgxpool.Pool
	SQL  *sql.DB
	DSN  string
}

// MigrationsDir resolves db/migrations from this file's own location, so a
// caller's package directory does not decide whether the path is right.
func MigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations")
}

// Start boots Postgres, applies every migration, and registers the teardown.
func Start(t *testing.T) *DB {
	t.Helper()

	dsn := startContainer(t)
	db := &DB{DSN: dsn, SQL: open(t, dsn)}

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	if err := goose.Up(db.SQL, MigrationsDir()); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	t.Cleanup(pool.Close)
	db.Pool = pool

	return db
}

func startContainer(t *testing.T) string {
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

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	handle, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Logf("close database: %v", err)
		}
	})

	return handle
}
