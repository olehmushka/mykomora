// Package httpserver assembles the HTTP surface: the chi router, the shared
// middleware chain, and the generated OpenAPI routes.
//
// Routes are never declared here by hand. They come from api/openapi.yaml via
// oapi-codegen, so the served surface and the published contract cannot drift.
package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// Server timeouts. readHeaderTimeout in particular is what keeps a slow-header
// client from holding a connection open indefinitely.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// NewRouter builds the router and mounts the generated OpenAPI handlers.
//
// Note the absence of chi's RealIP: it trusts X-Forwarded-For unconditionally
// and is therefore spoofable. Client IP is load-bearing here — it picks the
// default locale (M2) and scopes rate limits (M10) — so it gets a deliberate,
// proxy-aware resolver against Caddy's known address when those land, rather
// than a convenience middleware now.
func NewRouter(log *slog.Logger, handlers api.StrictServerInterface) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(requestLogger(log))
	router.Use(middleware.Recoverer)

	api.HandlerFromMux(api.NewStrictHandler(handlers, nil), router)

	return router
}

// NewServer wires the router into an http.Server with conservative timeouts.
func NewServer(cfg config.Config, router *chi.Mux) *http.Server {
	return &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// listenAndServe starts the listener synchronously so that a bind failure is
// reported as a startup error rather than swallowed by a background goroutine.
func listenAndServe(srv *http.Server, log *slog.Logger) error {
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", srv.Addr, err)
	}

	log.Info("http server listening", slog.String("addr", listener.Addr().String()))

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error("http server stopped unexpectedly", slog.String("error", err.Error()))
		}
	}()

	return nil
}

// shutdown drains in-flight requests, then closes the server.
func shutdown(ctx context.Context, srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down http server: %w", err)
	}

	return nil
}
