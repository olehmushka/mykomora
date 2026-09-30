/**
 * Session upkeep, in the one place that can do it.
 *
 * The access token lives fifteen minutes, so an ordinary page load frequently
 * arrives with an expired one and a perfectly good refresh token. Rotating it
 * means writing cookies, and a server component cannot write cookies during a
 * render — the proxy can, because it runs before the render and owns the
 * response.
 *
 * This is upkeep, not authorisation. Nothing here validates a token: core-api
 * does that on every call, and a forged `mk_session` gets past this file and
 * straight into a 401. What the redirect below buys is that a signed-out
 * visitor sees the sign-in page instead of an empty shell that 401s.
 *
 * `middleware.ts` was renamed to `proxy.ts` in Next 16; same mechanism.
 */
import { NextResponse, type NextRequest } from "next/server";

import { coreApiBaseUrl } from "@/lib/api/config";
import { REFRESH_COOKIE, SESSION_COOKIE } from "@/lib/api/cookies";

/** Routes that render without a session. Everything else needs one. */
const PUBLIC_PREFIXES = ["/sign-in", "/join"];

export const config = {
  // Static assets and core-api's own paths are none of this file's business —
  // and without the exclusion, auth logic would sit in front of every CSS file.
  matcher: ["/((?!api|healthz|_next/static|_next/image|favicon.ico).*)"],
};

export async function proxy(request: NextRequest): Promise<NextResponse> {
  const isPublic = PUBLIC_PREFIXES.some(
    (prefix) =>
      request.nextUrl.pathname === prefix || request.nextUrl.pathname.startsWith(`${prefix}/`),
  );

  if (request.cookies.has(SESSION_COOKIE)) {
    return NextResponse.next();
  }

  if (request.cookies.has(REFRESH_COOKIE)) {
    const refreshed = await rotate(request);
    if (refreshed) {
      return refreshed;
    }
    // The refresh token is spent, revoked, or was replayed. Fall through:
    // whatever cookies are left are dead, and core-api has already cleared
    // them on its own response.
  }

  if (isPublic) {
    return NextResponse.next();
  }

  return NextResponse.redirect(signInURL(request));
}

/**
 * Exchanges the refresh cookie for a new pair.
 *
 * On success the new cookies go two ways at once: onto the response, so the
 * browser keeps them, and onto the forwarded request, so this very render
 * already sees the fresh session instead of 401ing its way through.
 */
async function rotate(request: NextRequest): Promise<NextResponse | null> {
  let response: Response;

  try {
    response = await fetch(`${coreApiBaseUrl()}/api/v1/auth/refresh`, {
      method: "POST",
      headers: { Cookie: request.headers.get("cookie") ?? "" },
      cache: "no-store",
    });
  } catch {
    // core-api is unreachable. Treating that as a failed refresh would sign
    // everyone out of a stack that is merely still booting.
    return null;
  }

  if (!response.ok) {
    return null;
  }

  const setCookies = response.headers.getSetCookie();
  if (setCookies.length === 0) {
    return null;
  }

  const headers = new Headers(request.headers);
  headers.set("cookie", mergeCookieHeader(request.headers.get("cookie"), setCookies));

  const next = NextResponse.next({ request: { headers } });
  for (const cookie of setCookies) {
    next.headers.append("set-cookie", cookie);
  }

  return next;
}

/**
 * Rewrites the request's Cookie header with the freshly issued values.
 *
 * Only the name and value of each Set-Cookie matter here: attributes describe
 * how the browser should store a cookie, and this header is being handed
 * straight back to our own server.
 *
 * Exported for its test: getting this wrong sends the render the *old* access
 * token, which fails quietly as a 401 on a page that looks like it should
 * have worked.
 */
export function mergeCookieHeader(existing: string | null, setCookies: string[]): string {
  const jar = new Map<string, string>();

  for (const pair of existing?.split(";") ?? []) {
    const [name, ...rest] = pair.trim().split("=");
    if (name && rest.length > 0) {
      jar.set(name, rest.join("="));
    }
  }

  for (const cookie of setCookies) {
    const [nameValue] = cookie.split(";");
    const [name, ...rest] = (nameValue ?? "").split("=");
    if (name && rest.length > 0) {
      jar.set(name.trim(), rest.join("="));
    }
  }

  return [...jar].map(([name, value]) => `${name}=${value}`).join("; ");
}

function signInURL(request: NextRequest): URL {
  const target = new URL("/sign-in", request.url);

  // Where to come back to. core-api re-checks this on the way out, so a
  // tampered value cannot turn sign-in into an open redirect.
  const next = request.nextUrl.pathname + request.nextUrl.search;
  if (next !== "/") {
    target.searchParams.set("next", next);
  }

  return target;
}
