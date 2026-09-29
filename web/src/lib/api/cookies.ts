/**
 * The cookie names core-api sets, and the header that carries the CSRF token
 * back.
 *
 * Declared here rather than inlined because three unrelated places need to
 * agree on them: the proxy that refreshes sessions, the server-side fetch
 * wrapper that forwards them, and the sign-out action that clears them. They
 * are part of core-api's contract; `core-api/internal/auth/cookies.go` is the
 * other end of it.
 */
export const SESSION_COOKIE = "mk_session";
export const REFRESH_COOKIE = "mk_refresh";
export const CSRF_COOKIE = "mk_csrf";
export const CSRF_HEADER = "X-CSRF-Token";
