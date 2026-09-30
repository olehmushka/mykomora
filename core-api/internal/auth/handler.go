package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// signInPath is where a failed sign-in lands. The reason travels as a query
// parameter and is rendered by the web app, so the message the user reads is
// translated there rather than hardcoded in Go.
const signInPath = "/sign-in"

// Handler serves the auth operations of the generated contract.
type Handler struct {
	service *Service
	google  *GoogleProvider
	store   Store
	cookies cookieJar
	flow    flowCodec
	baseURL string
	log     *slog.Logger
}

// NewHandler builds the auth handler.
func NewHandler(
	service *Service,
	google *GoogleProvider,
	store Store,
	cfg config.Config,
	log *slog.Logger,
) *Handler {
	return &Handler{
		service: service,
		google:  google,
		store:   store,
		cookies: cookieJar{secure: cfg.CookieSecure()},
		flow:    flowCodec{secret: []byte(cfg.AppSecret), now: time.Now},
		baseURL: strings.TrimSuffix(cfg.PublicBaseURL, "/"),
		log:     log.With(slog.String("component", "auth.handler")),
	}
}

// StartGoogleAuth begins the sign-in flow.
func (h *Handler) StartGoogleAuth(
	ctx context.Context,
	request api.StartGoogleAuthRequestObject,
) (api.StartGoogleAuthResponseObject, error) {
	flow, err := h.flow.newFlow(
		optionalParam(request.Params.Invite),
		safeNextPath(optionalParam(request.Params.Next)),
	)
	if err != nil {
		return nil, err
	}

	consentURL, err := h.google.AuthCodeURL(ctx, flow)
	if err != nil {
		return h.googleUnavailable(ctx, err), nil
	}

	cookie, err := h.flow.encode(flow)
	if err != nil {
		return nil, err
	}

	return redirectWithCookies{
		location: consentURL,
		cookies:  []*http.Cookie{h.cookies.flow(cookie)},
	}, nil
}

// googleUnavailable sends the browser back to the sign-in page rather than
// answering with a status code.
//
// A deployment with no Google client is a supported state — a fresh clone runs
// the whole stack without one — and the person in front of the browser is
// owed a page that explains itself, not a JSON body rendered raw. The
// operator's signal is the log line.
func (h *Handler) googleUnavailable(ctx context.Context, err error) redirectWithCookies {
	if errors.Is(err, ErrGoogleNotConfigured) {
		h.log.WarnContext(ctx, "sign-in attempted without a google client configured")
	} else {
		h.log.ErrorContext(ctx, "google discovery failed", slog.String("error", err.Error()))
	}

	return h.signInFailed("unavailable")
}

// HandleGoogleCallback completes the sign-in flow.
//
// Failures a person can cause — declining consent, following a stale link,
// reloading a spent callback — redirect back to the sign-in page with a
// reason. Only a caller that is clearly not a browser gets a status code.
func (h *Handler) HandleGoogleCallback(
	ctx context.Context,
	request api.HandleGoogleCallbackRequestObject,
) (api.HandleGoogleCallbackResponseObject, error) {
	if optionalParam(request.Params.Error) != "" {
		// Google reports a declined consent screen this way. It is a choice,
		// not a fault.
		return h.signInFailed("denied"), nil
	}

	code := optionalParam(request.Params.Code)
	state := optionalParam(request.Params.State)

	if code == "" || state == "" {
		return api.HandleGoogleCallback400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Code:    "invalid_callback",
			Message: "the callback is missing its code or state",
		}}, nil
	}

	meta := RequestMetaFrom(ctx)

	flow, err := h.flow.decode(meta.Flow)
	if err != nil || flow.State != state {
		// Either the flow cookie is gone, or `state` does not match it. Both
		// mean this callback cannot be tied to a sign-in we started.
		return h.signInFailed("expired"), nil
	}

	identity, err := h.google.Exchange(ctx, code, flow.Verifier)
	if err != nil {
		return h.exchangeFailed(ctx, err), nil
	}

	session, err := h.service.SignIn(ctx, identity, flow.Invite, meta)
	if err != nil {
		return nil, fmt.Errorf("sign in: %w", err)
	}

	cookies := append(h.cookies.session(session), h.cookies.clearFlow())

	return redirectWithCookies{
		location: h.baseURL + safeNextPath(flow.Next),
		cookies:  cookies,
	}, nil
}

func (h *Handler) exchangeFailed(ctx context.Context, err error) redirectWithCookies {
	switch {
	case errors.Is(err, ErrGoogleNotConfigured):
		return h.googleUnavailable(ctx, err)
	case errors.Is(err, ErrUnverifiedEmail):
		// Google knows who they are but has not confirmed the address, and
		// the address is what a family recognises a member by.
		return h.signInFailed("unverified_email")
	}

	h.log.ErrorContext(ctx, "google code exchange failed", slog.String("error", err.Error()))

	return h.signInFailed("exchange_failed")
}

// signInFailed sends the browser back to the sign-in page with a reason code.
func (h *Handler) signInFailed(reason string) redirectWithCookies {
	target := h.baseURL + signInPath + "?error=" + url.QueryEscape(reason)

	return redirectWithCookies{
		location: target,
		cookies:  []*http.Cookie{h.cookies.clearFlow()},
	}
}

// RefreshSession rotates the session.
func (h *Handler) RefreshSession(
	ctx context.Context,
	_ api.RefreshSessionRequestObject,
) (api.RefreshSessionResponseObject, error) {
	meta := RequestMetaFrom(ctx)

	session, err := h.service.Refresh(ctx, meta.RefreshToken, meta)

	switch {
	case errors.Is(err, ErrTokenReuse):
		// Answered exactly like an expired session, and with the cookies
		// cleared as well as the tokens revoked. A distinct response would
		// tell whoever replayed the token that the replay was noticed; the
		// client's only correct next move is to sign in again either way.
		return unauthorizedWithCookies{cookies: h.cookies.clearSession()}, nil
	case errors.Is(err, ErrNoSession):
		return api.RefreshSession401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{
			Code:    "no_session",
			Message: "the session has expired; sign in again",
		}}, nil
	case err != nil:
		return nil, err
	}

	return noContentWithCookies{cookies: h.cookies.session(session)}, nil
}

// Logout revokes the presented refresh token and clears the cookies.
func (h *Handler) Logout(
	ctx context.Context,
	_ api.LogoutRequestObject,
) (api.LogoutResponseObject, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return api.Logout401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{
			Code:    "no_session",
			Message: "there is no session to end",
		}}, nil
	}

	if err := h.service.Logout(ctx, principal, RequestMetaFrom(ctx).RefreshToken); err != nil {
		return nil, err
	}

	return noContentWithCookies{cookies: h.cookies.clearSession()}, nil
}

// GetMe reports who the session belongs to and which family it is scoped to.
func (h *Handler) GetMe(
	ctx context.Context,
	_ api.GetMeRequestObject,
) (api.GetMeResponseObject, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return api.GetMe401JSONResponse{UnauthorizedJSONResponse: noSession()}, nil
	}

	user, err := h.store.GetUserByID(ctx, pgconv.UUID(principal.UserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.GetMe401JSONResponse{UnauthorizedJSONResponse: noSession()}, nil
		}

		return nil, fmt.Errorf("load user: %w", err)
	}

	family, err := h.store.GetFamily(ctx, db.GetFamilyParams{
		FamilyID: pgconv.UUID(principal.FamilyID),
		UserID:   pgconv.UUID(principal.UserID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.GetMe401JSONResponse{UnauthorizedJSONResponse: noSession()}, nil
		}

		return nil, fmt.Errorf("load family: %w", err)
	}

	return api.GetMe200JSONResponse{
		User:   apimap.User(user),
		Family: apimap.Family(family),
		Role:   api.FamilyRole(principal.Role),
	}, nil
}

// noSession is the standard 401 body. Every operation's 401 type wraps it.
func noSession() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{
		Code:    "no_session",
		Message: "sign in to continue",
	}
}

func optionalParam(v *string) string {
	if v == nil {
		return ""
	}

	return strings.TrimSpace(*v)
}
