import { redirect } from "next/navigation";

import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Me = components["schemas"]["Me"];
export type User = components["schemas"]["User"];
export type Family = components["schemas"]["Family"];
export type FamilyRole = components["schemas"]["FamilyRole"];

/** Who the session belongs to, or a reason there is nobody. */
export function getMe() {
  type Ok = operations["getMe"]["responses"][200]["content"]["application/json"];

  return callApi<Ok>("/api/v1/me");
}

/** Ends the session: core-api revokes the refresh token and clears the cookies. */
export function logout() {
  return callApi<void>("/api/v1/auth/logout", { method: "POST" });
}

/**
 * The session, or a redirect to sign-in.
 *
 * The proxy already turns away unauthenticated requests for protected routes,
 * so reaching this with no session means the token expired between the proxy
 * and the render, or was revoked. Either way the page has nothing to draw.
 */
export async function requireMe(currentPath: string): Promise<Me> {
  const result = await getMe();

  if (result.ok) {
    return result.data;
  }

  if (result.reason === "unauthenticated") {
    redirect(`/sign-in?next=${encodeURIComponent(currentPath)}`);
  }

  // Anything else is core-api being unwell rather than the user being
  // unwelcome, and throwing renders the nearest error boundary.
  throw new Error(result.detail);
}
