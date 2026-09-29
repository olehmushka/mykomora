package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Module provides the connection pool and the generated query interface, and
// ties the pool's health check and teardown to the application lifecycle.
var Module = fx.Module("postgres",
	fx.Provide(NewPool, NewQuerier, NewTxRunner),
	fx.Invoke(func(lc fx.Lifecycle, pool *pgxpool.Pool, log *slog.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := Ping(ctx, pool); err != nil {
					return err
				}
				log.Info("postgres ready")

				return nil
			},
			OnStop: func(context.Context) error {
				pool.Close()
				log.Info("postgres pool closed")

				return nil
			},
		})
	}),
)
