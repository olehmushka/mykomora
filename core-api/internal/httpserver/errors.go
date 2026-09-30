package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
)

// The generated strict server turns a handler that returns an error into a
// call to one of these. Without them every failure would be a plain-text 500,
// including the two that are not failures at all: an unauthenticated request
// and a missing CSRF token, which the session middleware reports as errors
// because that is the only channel a strict middleware has.

// requestErrorHandler answers a request the generated code could not even
// decode — an unparseable path parameter, a malformed body.
func requestErrorHandler(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		log.DebugContext(r.Context(), "rejected malformed request",
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)

		writeError(w, r, http.StatusBadRequest, "invalid_request", "the request could not be read")
	}
}

// responseErrorHandler maps an error returned by a handler or by the session
// middleware onto a status code and a stable error code.
func responseErrorHandler(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		switch {
		case errors.Is(err, auth.ErrUnauthenticated):
			writeError(w, r, http.StatusUnauthorized, "no_session", "sign in to continue")
		case errors.Is(err, auth.ErrCSRF):
			writeError(w, r, http.StatusForbidden, "csrf_failed",
				"this request is missing its CSRF token")
		default:
			// Everything else is a fault on our side. It is logged in full and
			// reported as nothing: an internal error message is for the
			// operator, not for the client.
			log.ErrorContext(r.Context(), "unhandled error serving request",
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("path", r.URL.Path),
				slog.String("error", err.Error()),
			)

			writeError(w, r, http.StatusInternalServerError, "internal_error",
				"something went wrong on our side")
		}
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(api.Error{Code: code, Message: message}); err != nil {
		// The status line is already on the wire, so there is nothing left to
		// say to the client; the request log carries the status either way.
		_ = r
	}
}
