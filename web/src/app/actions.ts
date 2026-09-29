"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { CSRF_COOKIE, REFRESH_COOKIE, SESSION_COOKIE } from "@/lib/api/cookies";
import { logout } from "@/lib/api/session";

/**
 * Ends the session on both sides.
 *
 * core-api revokes the refresh token and clears its own cookies — but on its
 * own response, which the browser never sees, because this call is made from
 * the Next server. So the cookies are cleared again here, on the response the
 * browser *does* see. Revoking without clearing would leave a signed-out
 * browser holding dead cookies; clearing without revoking would leave a live
 * refresh token in the database.
 */
export async function signOut(): Promise<never> {
  await logout();

  const jar = await cookies();
  for (const name of [SESSION_COOKIE, REFRESH_COOKIE, CSRF_COOKIE]) {
    jar.delete(name);
  }

  redirect("/sign-in");
}
