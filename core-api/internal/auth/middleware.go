package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// Middleware errors, mapped to status codes by the HTTP layer.
var (
	// ErrUnauthenticated means the operation needs a session and the request
	// did not carry a usable one.
	ErrUnauthenticated = errors.New("authentication required")

	// ErrCSRF means a cookie-authenticated mutation did not echo the
	// double-submit token.
	ErrCSRF = errors.New("csrf token missing or mismatched")
)

// publicOperations are the operations that intentionally serve without a
// session, by generated operation ID.
//
// This is the authentication counterpart of the `family-scoped` allowlist in
// sqlc.yaml, and it exists for the same reason: opening a hole should be a
// line in a reviewed list, not an omission nobody notices. Anything absent
// here requires a session.
//
// TestPublicOperationsMatchTheSpec keeps this in step with the `security: []`
// declarations in api/openapi.yaml, so the contract and the code cannot drift.
var publicOperations = map[string]struct{}{
	"GetHealthz":           {}, // liveness, no tenant data
	"GetPing":              {}, // round-trip probe, no tenant data
	"StartGoogleAuth":      {}, // signing in is what you do without a session
	"HandleGoogleCallback": {}, // ditto, and the session is issued here
	"RefreshSession":       {}, // authenticated by the refresh cookie itself
	"GetInvitePreview":     {}, // the invitee has no account yet
}

// PublicOperations lists the operations served without a session, sorted.
// Exported for the test that compares this list against the OpenAPI document.
func PublicOperations() []string {
	names := make([]string, 0, len(publicOperations))
	for name := range publicOperations {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// Middleware authenticates requests and enforces CSRF on cookie-driven
// mutations.
type Middleware struct {
	tokens *tokenIssuer
	log    *slog.Logger
}

// NewMiddleware builds the session middleware.
func NewMiddleware(cfg config.Config, log *slog.Logger) *Middleware {
	return &Middleware{
		tokens: newTokenIssuer(cfg),
		log:    log.With(slog.String("component", "auth.middleware")),
	}
}

// Strict returns the middleware in the form oapi-codegen's strict server
// expects.
//
// It runs per operation and knows the operation's ID, which is what makes a
// declarative public list possible: a plain chi middleware runs before routing
// and would have to re-derive the operation from the method and path.
func (m *Middleware) Strict() api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(
			ctx context.Context,
			w http.ResponseWriter,
			r *http.Request,
			request any,
		) (any, error) {
			meta := readRequestMeta(r)

			principal, authenticated := m.resolve(r, &meta)
			if authenticated {
				ctx = WithPrincipal(ctx, principal)
			}

			ctx = WithRequestMeta(ctx, meta)

			if _, public := publicOperations[operationID]; public {
				return next(ctx, w, r, request)
			}

			if !authenticated {
				return nil, ErrUnauthenticated
			}

			if err := checkCSRF(r, meta); err != nil {
				return nil, err
			}

			return next(ctx, w, r, request)
		}
	}
}

// resolve recovers the principal from the request, preferring the bearer
// header so that a mobile client is never confused by a stale browser cookie.
func (m *Middleware) resolve(r *http.Request, meta *RequestMeta) (Principal, bool) {
	if raw, ok := bearerToken(r); ok {
		principal, err := m.tokens.parseAccess(raw)
		if err != nil {
			return Principal{}, false
		}

		return principal, true
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Principal{}, false
	}

	principal, err := m.tokens.parseAccess(cookie.Value)
	if err != nil {
		// Expected and frequent: an access token lives fifteen minutes, so an
		// idle tab produces one of these on every wake-up. The client is meant
		// to answer a 401 by refreshing, so this is not worth a log line above
		// debug.
		m.log.Debug("rejected session cookie", slog.String("error", err.Error()))

		return Principal{}, false
	}

	meta.CookieAuthenticated = true

	return principal, true
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")

	value, found := strings.CutPrefix(header, "Bearer ")
	if !found {
		return "", false
	}

	value = strings.TrimSpace(value)

	return value, value != ""
}

// checkCSRF enforces the double-submit token on cookie-authenticated
// mutations.
//
// Bearer-authenticated requests are exempt: a browser never attaches an
// Authorization header on its own, so there is nothing for another origin to
// forge. Safe methods are exempt because they do not change anything.
func checkCSRF(r *http.Request, meta RequestMeta) error {
	if !meta.CookieAuthenticated || isSafeMethod(r.Method) {
		return nil
	}

	if meta.CSRFCookie == "" || meta.CSRFHeader == "" {
		return ErrCSRF
	}

	if subtle.ConstantTimeCompare([]byte(meta.CSRFCookie), []byte(meta.CSRFHeader)) != 1 {
		return ErrCSRF
	}

	return nil
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
