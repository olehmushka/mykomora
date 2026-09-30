package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// sentinel is what the wrapped handler returns when it is reached at all.
type sentinel struct {
	principal Principal
	reached   bool
}

func (m *Middleware) wrap(operationID string, seen *sentinel) api.StrictHandlerFunc {
	next := func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		seen.reached = true
		seen.principal, _ = PrincipalFrom(ctx)

		return nil, nil
	}

	return m.Strict()(next, operationID)
}

func TestPublicOperationsNeedNoSession(t *testing.T) {
	t.Parallel()

	m := NewMiddleware(testConfig(), discardLogger())

	for _, operation := range PublicOperations() {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			var seen sentinel

			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

			if _, err := m.wrap(operation, &seen)(r.Context(), httptest.NewRecorder(), r, nil); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if !seen.reached {
				t.Error("the handler was not reached")
			}
		})
	}
}

func TestProtectedOperationRequiresASession(t *testing.T) {
	t.Parallel()

	m := NewMiddleware(testConfig(), discardLogger())
	issuer := newTokenIssuer(testConfig())

	valid, _, err := issuer.issueAccess(testPrincipal())
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	tests := []struct {
		name    string
		prepare func(*http.Request)
		want    error
	}{
		{
			name:    "no credentials at all",
			prepare: func(*http.Request) {},
			want:    ErrUnauthenticated,
		},
		{
			name: "a cookie that is not a token",
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "nonsense"})
			},
			want: ErrUnauthenticated,
		},
		{
			name: "a bearer token that is not a token",
			prepare: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer nonsense")
			},
			want: ErrUnauthenticated,
		},
		{
			name: "a valid session cookie",
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: valid})
			},
			want: nil,
		},
		{
			name: "a valid bearer token",
			prepare: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+valid)
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var seen sentinel

			r := httptest.NewRequest(http.MethodGet, "/api/v1/me", http.NoBody)
			tc.prepare(r)

			_, err := m.wrap("GetMe", &seen)(r.Context(), httptest.NewRecorder(), r, nil)

			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}

			if reached := tc.want == nil; seen.reached != reached {
				t.Fatalf("handler reached = %v, want %v", seen.reached, reached)
			}

			if tc.want == nil && seen.principal != testPrincipal() {
				t.Errorf("principal = %+v, want %+v", seen.principal, testPrincipal())
			}
		})
	}
}

func TestCSRFOnCookieAuthenticatedMutations(t *testing.T) {
	t.Parallel()

	m := NewMiddleware(testConfig(), discardLogger())
	issuer := newTokenIssuer(testConfig())

	token, _, err := issuer.issueAccess(testPrincipal())
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	const csrf = "a-double-submit-token"

	tests := []struct {
		name    string
		method  string
		prepare func(*http.Request)
		want    error
	}{
		{
			name:   "cookie read needs no csrf token",
			method: http.MethodGet,
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
			},
		},
		{
			name:   "cookie mutation with a matching token",
			method: http.MethodPatch,
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
				r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
				r.Header.Set(CSRFHeaderName, csrf)
			},
		},
		{
			name:   "cookie mutation with no header",
			method: http.MethodPatch,
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
				r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
			},
			want: ErrCSRF,
		},
		{
			name:   "cookie mutation with a mismatched header",
			method: http.MethodPatch,
			prepare: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
				r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
				r.Header.Set(CSRFHeaderName, "something-else")
			},
			want: ErrCSRF,
		},
		{
			name:   "bearer mutation is exempt",
			method: http.MethodPatch,
			prepare: func(r *http.Request) {
				// No cookie is involved, so no other origin can cause this
				// request in the first place.
				r.Header.Set("Authorization", "Bearer "+token)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var seen sentinel

			r := httptest.NewRequest(tc.method, "/api/v1/family", http.NoBody)
			tc.prepare(r)

			_, err := m.wrap("UpdateFamily", &seen)(r.Context(), httptest.NewRecorder(), r, nil)

			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}

			if reached := tc.want == nil; seen.reached != reached {
				t.Errorf("handler reached = %v, want %v", seen.reached, reached)
			}
		})
	}
}

func TestRequestMetaReachesTheHandler(t *testing.T) {
	t.Parallel()

	m := NewMiddleware(testConfig(), discardLogger())

	var meta RequestMeta

	next := func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		meta = RequestMetaFrom(ctx)

		return nil, nil
	}

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", http.NoBody)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("User-Agent", "test-agent")
	r.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "a-refresh-token"})

	if _, err := m.Strict()(next, "RefreshSession")(r.Context(), httptest.NewRecorder(), r, nil); err != nil {
		t.Fatalf("error = %v, want nil", err)
	}

	if meta.RefreshToken != "a-refresh-token" {
		t.Errorf("refresh token = %q, want it read from the cookie", meta.RefreshToken)
	}

	if meta.UserAgent != "test-agent" {
		t.Errorf("user agent = %q, want %q", meta.UserAgent, "test-agent")
	}

	if meta.IP == nil || meta.IP.String() != "203.0.113.7" {
		t.Errorf("ip = %v, want 203.0.113.7", meta.IP)
	}
}
