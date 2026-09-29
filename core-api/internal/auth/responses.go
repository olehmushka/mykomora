package auth

import "net/http"

// The generated strict-server response objects cannot set cookies: they are
// plain structs, and the generated Visit method writes only a status, headers
// declared in the spec, and a body.
//
// The response interfaces are structural, though — a type in this package that
// has the right Visit method satisfies them. So sign-in and session rotation
// return their own response objects, and keep the Set-Cookie logic in the one
// package that owns sessions rather than pushing http.ResponseWriter down into
// every handler signature.

// redirectWithCookies is a 302 that also writes cookies.
type redirectWithCookies struct {
	location string
	cookies  []*http.Cookie
}

func (r redirectWithCookies) write(w http.ResponseWriter) error {
	for _, cookie := range r.cookies {
		http.SetCookie(w, cookie)
	}

	w.Header().Set("Location", r.location)
	w.WriteHeader(http.StatusFound)

	return nil
}

// VisitStartGoogleAuthResponse satisfies api.StartGoogleAuthResponseObject.
func (r redirectWithCookies) VisitStartGoogleAuthResponse(w http.ResponseWriter) error {
	return r.write(w)
}

// VisitHandleGoogleCallbackResponse satisfies api.HandleGoogleCallbackResponseObject.
func (r redirectWithCookies) VisitHandleGoogleCallbackResponse(w http.ResponseWriter) error {
	return r.write(w)
}

// noContentWithCookies is a 204 that also writes cookies.
type noContentWithCookies struct {
	cookies []*http.Cookie
}

func (n noContentWithCookies) write(w http.ResponseWriter) error {
	for _, cookie := range n.cookies {
		http.SetCookie(w, cookie)
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// VisitRefreshSessionResponse satisfies api.RefreshSessionResponseObject.
func (n noContentWithCookies) VisitRefreshSessionResponse(w http.ResponseWriter) error {
	return n.write(w)
}

// VisitLogoutResponse satisfies api.LogoutResponseObject.
func (n noContentWithCookies) VisitLogoutResponse(w http.ResponseWriter) error {
	return n.write(w)
}

// unauthorizedWithCookies is a 401 that also clears cookies. Session rotation
// uses it when a refresh token turns out to have been replayed: the tokens are
// revoked in the database, and the browser is emptied on the way out.
type unauthorizedWithCookies struct {
	cookies []*http.Cookie
}

// VisitRefreshSessionResponse satisfies api.RefreshSessionResponseObject.
func (u unauthorizedWithCookies) VisitRefreshSessionResponse(w http.ResponseWriter) error {
	for _, cookie := range u.cookies {
		http.SetCookie(w, cookie)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_, err := w.Write([]byte(`{"code":"no_session","message":"the session has expired; sign in again"}`))

	return err
}
