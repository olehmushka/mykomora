// Package auth owns identity for the whole system: the Google OAuth2 exchange,
// ID-token verification, this API's own access and refresh tokens, and the
// middleware that turns a request into a family-scoped principal.
//
// It lives in core-api and nowhere else. The web app forwards credentials and
// never mints or validates them, because the moment identity moves into
// Next.js the planned mobile client inherits a rewrite (SPEC 6, "core-api owns
// identity").
package auth

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/google/uuid"
)

// Role mirrors the `family_role` enum. Both roles see and edit everything;
// `owner` additionally administers the family (SPEC 3, "Concepts").
type Role string

// The roles a member can hold.
const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

// Principal is who the request is acting as, and inside which tenant.
//
// FamilyID is the whole point: it comes from the verified session and is the
// value every family-scoped query is given. A handler must never take a family
// id from the request body or the path.
type Principal struct {
	UserID   uuid.UUID
	FamilyID uuid.UUID
	Role     Role
}

// IsOwner reports whether this principal may administer the family.
func (p Principal) IsOwner() bool { return p.Role == RoleOwner }

// RequestMeta is the part of the HTTP request the generated strict handlers do
// not receive.
//
// oapi-codegen's strict interface hands a handler a typed request object and a
// context, which is what makes those handlers pleasant to test — but cookies,
// the user agent and the client address are not in it. The session middleware
// reads them once and puts this in the context, so handlers stay free of
// *http.Request without anyone reaching for a global.
type RequestMeta struct {
	UserAgent string
	IP        *netip.Addr

	// RefreshToken is the raw value of the refresh cookie, empty when absent.
	RefreshToken string
	// Flow is the raw value of the OAuth flow cookie, empty when absent.
	Flow string

	// CookieAuthenticated records that the principal came from a cookie rather
	// than from an Authorization header. Only cookie-driven requests are
	// subject to the CSRF check: a bearer token is not attached by the browser
	// automatically, so there is nothing to forge.
	CookieAuthenticated bool
	CSRFCookie          string
	CSRFHeader          string
}

type contextKey int

const (
	principalKey contextKey = iota
	requestMetaKey
)

// WithPrincipal returns a context carrying the authenticated principal.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom returns the authenticated principal, if the request had one.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)

	return p, ok
}

// WithRequestMeta returns a context carrying the request metadata.
func WithRequestMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaKey, m)
}

// RequestMetaFrom returns the request metadata the middleware recorded. A
// zero value means the request did not pass through the middleware, which in
// practice only happens in a test.
func RequestMetaFrom(ctx context.Context) RequestMeta {
	m, ok := ctx.Value(requestMetaKey).(RequestMeta)
	if !ok {
		return RequestMeta{}
	}

	return m
}

// readRequestMeta extracts everything the handlers need from the raw request.
func readRequestMeta(r *http.Request) RequestMeta {
	meta := RequestMeta{
		UserAgent:  r.UserAgent(),
		IP:         clientIP(r),
		CSRFHeader: r.Header.Get(CSRFHeaderName),
	}

	if c, err := r.Cookie(RefreshCookieName); err == nil {
		meta.RefreshToken = c.Value
	}

	if c, err := r.Cookie(FlowCookieName); err == nil {
		meta.Flow = c.Value
	}

	if c, err := r.Cookie(CSRFCookieName); err == nil {
		meta.CSRFCookie = c.Value
	}

	return meta
}

// clientIP resolves the peer address, deliberately without consulting
// X-Forwarded-For.
//
// That header is trivially spoofable and nothing in front of core-api is
// trusted to sanitise it yet. Client IP becomes load-bearing in M2 (default
// locale) and M10 (rate limits); both deserve a proxy-aware resolver keyed to
// Caddy's known address, not a guess made here.
func clientIP(r *http.Request) *netip.Addr {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return nil
	}

	return &addr
}
