package auth

import (
	"net/http"
	"time"
)

// Cookie and header names. `mk_` keeps them recognisable in a browser's
// storage inspector next to whatever else is on localhost.
const (
	// SessionCookieName holds the access JWT.
	SessionCookieName = "mk_session"
	// RefreshCookieName holds the opaque refresh token.
	RefreshCookieName = "mk_refresh"
	// CSRFCookieName holds the double-submit token. Readable by scripts on
	// purpose — the web app has to echo it back in a header.
	CSRFCookieName = "mk_csrf"
	// FlowCookieName holds the in-progress OAuth state and PKCE verifier.
	FlowCookieName = "mk_oauth"
	// CSRFHeaderName is where the double-submit token comes back.
	CSRFHeaderName = "X-CSRF-Token"
)

// cookieJar builds the session cookies.
//
// Every cookie is written with Path=/ rather than scoped to /api. It has to
// be: the Next.js server reads them on ordinary page requests and forwards
// them to core-api, and a cookie scoped to /api is simply not sent with a
// request for /settings/family.
type cookieJar struct {
	secure bool
}

func (j cookieJar) base(name, value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:  name,
		Value: value,
		Path:  "/",
		// Lax rather than Strict: the sign-in redirect arrives from Google as
		// a cross-site top-level navigation, and Strict would drop the very
		// cookies that redirect is there to set.
		SameSite: http.SameSiteLaxMode,
		Secure:   j.secure,
		HttpOnly: true,
		MaxAge:   int(ttl.Seconds()),
	}
}

// session builds the three cookies that make up a signed-in browser.
func (j cookieJar) session(s Session) []*http.Cookie {
	csrf := j.base(CSRFCookieName, s.CSRFToken, time.Until(s.RefreshExpiry))
	// The CSRF token is the one value the page's own code must read, so that
	// it can send it back in a header. That is the whole double-submit trick:
	// script on this origin can read it, a cross-site request cannot.
	csrf.HttpOnly = false

	return []*http.Cookie{
		j.base(SessionCookieName, s.AccessToken, time.Until(s.AccessExpiry)),
		j.base(RefreshCookieName, s.RefreshToken, time.Until(s.RefreshExpiry)),
		csrf,
	}
}

// clearSession expires the session cookies.
func (j cookieJar) clearSession() []*http.Cookie {
	names := []string{SessionCookieName, RefreshCookieName, CSRFCookieName}
	cookies := make([]*http.Cookie, 0, len(names))

	for _, name := range names {
		c := j.base(name, "", 0)
		c.MaxAge = -1
		c.Expires = time.Unix(0, 0)
		cookies = append(cookies, c)
	}

	return cookies
}

// flow builds the short-lived cookie that carries an in-progress sign-in.
func (j cookieJar) flow(value string) *http.Cookie {
	return j.base(FlowCookieName, value, flowTTL)
}

// clearFlow expires the sign-in cookie once the callback has consumed it.
func (j cookieJar) clearFlow() *http.Cookie {
	c := j.base(FlowCookieName, "", 0)
	c.MaxAge = -1
	c.Expires = time.Unix(0, 0)

	return c
}
