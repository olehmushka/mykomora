/**
 * The server-side half of the API client: it knows how to get the browser's
 * cookies and hand them to core-api.
 *
 * Kept apart from `client.ts` on purpose. This module reaches for
 * `next/headers` and therefore only works inside a request — which is also
 * what keeps it off the client, since importing it from a client component is
 * a build error. The transport underneath it is a plain function that a test
 * can call with a string, and that split is what lets the wrapper be tested
 * without a Next runtime.
 */
import { cookies } from "next/headers";

import { apiRequest, type ApiResult, type RequestOptions } from "@/lib/api/client";
import { CSRF_COOKIE, SESSION_COOKIE } from "@/lib/api/cookies";

type ServerRequestOptions = Omit<RequestOptions, "cookieHeader" | "csrfToken">;

/**
 * Calls core-api as the signed-in user.
 *
 * Every cookie the browser sent is forwarded, not just the session one: the
 * refresh cookie has to reach `/auth/logout`, and forwarding the lot keeps
 * this function from having to know which endpoint wants which.
 *
 * The CSRF token is attached to mutations only, mirroring the check on the
 * other side. It is read from the cookie the browser also holds — the whole
 * point of double-submit is that only script on this origin can do both.
 */
export async function callApi<T>(
  path: string,
  options: ServerRequestOptions = {},
): Promise<ApiResult<T>> {
  const jar = await cookies();
  const method = options.method ?? "GET";

  return apiRequest<T>(path, {
    ...options,
    cookieHeader: jar.toString(),
    csrfToken: method === "GET" ? undefined : jar.get(CSRF_COOKIE)?.value,
  });
}

/**
 * Whether this request carries a session at all.
 *
 * Only a hint: the token is verified by core-api, never here. It saves a round
 * trip on pages that would otherwise ask `/me` just to find out there is
 * nobody to ask about.
 */
export async function hasSessionCookie(): Promise<boolean> {
  const jar = await cookies();

  return jar.has(SESSION_COOKIE);
}
