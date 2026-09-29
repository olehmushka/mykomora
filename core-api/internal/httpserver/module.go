package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// Module provides the router and HTTP server, and binds them to the application
// lifecycle so `fx` owns startup ordering and graceful shutdown.
var Module = fx.Module("httpserver",
	fx.Provide(NewRouter, NewServer),
	fx.Invoke(func(lc fx.Lifecycle, srv *http.Server, cfg config.Config, log *slog.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				return listenAndServe(srv, log)
			},
			OnStop: func(ctx context.Context) error {
				log.Info("http server shutting down")

				return shutdown(ctx, srv, cfg.ShutdownTimeout)
			},
		})
	}),
)
