// Command migrate runs goose migrations as a library rather than the stock
// CLI.
//
// It exists because of db/migrations' one Go-function migration
// (00006_taxonomy_seed.go): goose.AddMigrationContext only registers a
// migration with the process that imports the package defining it, and the
// stock `goose` CLI (what `make migrate-*` used before this command existed)
// never imports application code. This command does the one thing that CLI
// cannot: blank-import db/migrations so the Go migration is in goose's
// registry before Up/Down/Status run.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, for goose
	"github.com/pressly/goose/v3"

	_ "github.com/olehmushka/mykomora/core-api/db/migrations" // registers the Go migrations
)

const migrationsDir = "db/migrations"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|down-to-0|status>")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			fmt.Fprintln(os.Stderr, "close database:", cerr)
		}
	}()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "up":
		return goose.UpContext(ctx, db, migrationsDir)
	case "down":
		return goose.DownContext(ctx, db, migrationsDir)
	case "down-to-0":
		return goose.DownToContext(ctx, db, migrationsDir, 0)
	case "status":
		return goose.StatusContext(ctx, db, migrationsDir)
	default:
		return fmt.Errorf("unknown verb %q: want up, down, down-to-0 or status", os.Args[1])
	}
}
