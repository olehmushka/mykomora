package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/health"
)

// stubStore stands in for the generated queries so the handler can be tested
// without a database. It satisfies health.Store — the one query this handler
// is allowed to reach — which means this test stops compiling if that handler
// starts reaching for more.
type stubStore struct {
	now time.Time
	err error
}

var _ health.Store = stubStore{}

func (s stubStore) GetServerTime(context.Context) (pgtype.Timestamptz, error) {
	if s.err != nil {
		return pgtype.Timestamptz{}, s.err
	}

	return pgtype.Timestamptz{Time: s.now, Valid: true}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// systemServer is the generated strict interface with only the system
// operations filled in.
//
// The embedded interface is nil, so reaching any other operation panics —
// which is the right outcome for a test that must never reach one. In
// production the same interface is satisfied by internal/httpapi, which
// composes every domain handler.
type systemServer struct {
	api.StrictServerInterface

	handler *health.Handler
}

func (s systemServer) GetHealthz(
	ctx context.Context,
	request api.GetHealthzRequestObject,
) (api.GetHealthzResponseObject, error) {
	return s.handler.GetHealthz(ctx, request)
}

func (s systemServer) GetPing(
	ctx context.Context,
	request api.GetPingRequestObject,
) (api.GetPingResponseObject, error) {
	return s.handler.GetPing(ctx, request)
}

// newTestServer mounts the handler exactly the way production does: through the
// generated strict wrapper and the generated routes. Testing the handler in
// isolation would not prove that the contract and the code agree.
func newTestServer(t *testing.T, store health.Store) http.Handler {
	t.Helper()

	server := systemServer{handler: health.New(store, discardLogger())}

	return api.Handler(api.NewStrictHandler(server, nil))
}

func TestGetHealthzReportsOK(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, stubStore{now: time.Now()})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got api.HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got.Status != api.HealthStatusStatusOk {
		t.Errorf("status = %q, want %q", got.Status, api.HealthStatusStatusOk)
	}
}

// Liveness must not depend on Postgres: if it did, a transient database outage
// would get the process restarted instead of left alone to recover.
func TestGetHealthzIgnoresDatabaseFailure(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, stubStore{err: errors.New("connection refused")})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetPingReturnsDatabaseClock(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)

	srv := newTestServer(t, stubStore{now: want})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got api.PingResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if !got.Now.Equal(want) {
		t.Errorf("now = %s, want %s", got.Now, want)
	}

	if got.Database != api.PingResultDatabaseOk {
		t.Errorf("database = %q, want %q", got.Database, api.PingResultDatabaseOk)
	}
}

// A database that does not answer is a documented 503 in the contract, not a
// 500 and not a panic.
func TestGetPingReportsDatabaseUnavailable(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, stubStore{err: errors.New("connection refused")})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", http.NoBody))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var got api.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got.Code != "database_unavailable" {
		t.Errorf("code = %q, want %q", got.Code, "database_unavailable")
	}
}
