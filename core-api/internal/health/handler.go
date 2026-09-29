// Package health implements the operational endpoints: liveness and a
// round-trip probe through Postgres.
//
// These are the only handlers in core-api that serve no tenant data, and so the
// only ones exempt from resolving a family from the session.
package health

import (
	"context"
	"log/slog"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// codeDatabaseUnavailable is the stable error code clients match on when the
// database did not answer.
const codeDatabaseUnavailable = "database_unavailable"

// Handler serves the system endpoints declared in api/openapi.yaml.
type Handler struct {
	queries db.Querier
	log     *slog.Logger
}

// New builds the system endpoint handler.
func New(queries db.Querier, log *slog.Logger) *Handler {
	return &Handler{
		queries: queries,
		log:     log.With(slog.String("component", "health")),
	}
}

// GetHealthz reports that the process is serving. It deliberately does not
// touch the database: liveness must not fail because a dependency is down, or
// an orchestrator will restart a process that only needed to wait.
func (h *Handler) GetHealthz(
	_ context.Context,
	_ api.GetHealthzRequestObject,
) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse{Status: api.HealthStatusStatusOk}, nil
}

// GetPing exercises the whole request path — generated handler, generated query,
// Postgres — and returns the database's own clock.
func (h *Handler) GetPing(
	ctx context.Context,
	_ api.GetPingRequestObject,
) (api.GetPingResponseObject, error) {
	now, err := h.queries.GetServerTime(ctx)
	if err != nil {
		// Reported, not returned: an unreachable database is an expected
		// operational state with a documented 503 in the contract, not an
		// unhandled server fault.
		h.log.ErrorContext(ctx, "database probe failed", slog.String("error", err.Error()))

		return api.GetPing503JSONResponse{
			Code:    codeDatabaseUnavailable,
			Message: "the database did not answer",
		}, nil
	}

	return api.GetPing200JSONResponse{
		Now:      now.Time,
		Database: api.PingResultDatabaseOk,
	}, nil
}
