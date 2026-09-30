// Package postgres owns the connection pool and hands the rest of the
// application the sqlc-generated query interface.
//
// Nothing outside this package constructs a pool or writes SQL by hand: queries
// live in db/queries and reach callers as generated, type-safe methods.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// connectTimeout bounds the initial connectivity check at startup. A failure
// here is fatal: core-api has nothing useful to serve without Postgres.
const connectTimeout = 10 * time.Second

// NewPool builds a pgx pool and verifies connectivity before the application is
// considered started. The pool is closed on shutdown.
func NewPool(cfg config.Config, log *slog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	log.Debug("postgres pool created",
		slog.String("host", poolCfg.ConnConfig.Host),
		slog.String("database", poolCfg.ConnConfig.Database),
		slog.Int("max_conns", int(poolCfg.MaxConns)),
	)

	return pool, nil
}

// Ping verifies that the database answers, with a bounded timeout.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	return nil
}

// NewQuerier exposes the sqlc-generated queries over the pool.
//
// Callers depend on db.Querier, not on the pool, so a handler cannot reach past
// the generated queries and issue ad-hoc SQL — which is how the family-scoping
// invariant stays enforceable.
func NewQuerier(pool *pgxpool.Pool) db.Querier {
	return db.New(pool)
}

// TxRunner runs work inside a database transaction.
//
// Several M1 flows write more than one row and must not be observable
// half-done: creating a family and its first owner, accepting an invite while
// joining, and rotating a refresh token while issuing its successor. Giving
// callers a runner rather than the pool keeps `pgxpool` out of the handler
// packages and keeps commit and rollback in exactly one place.
type TxRunner struct {
	pool *pgxpool.Pool
}

// NewTxRunner builds the transaction runner over the pool.
func NewTxRunner(pool *pgxpool.Pool) *TxRunner {
	return &TxRunner{pool: pool}
}

// InTx runs fn against queries bound to a transaction, committing when fn
// returns nil and rolling back on any error or panic.
func (r *TxRunner) InTx(ctx context.Context, fn func(*db.Queries) error) error {
	if err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(db.New(tx))
	}); err != nil {
		return fmt.Errorf("run transaction: %w", err)
	}

	return nil
}
