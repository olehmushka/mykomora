"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { isLocale, NEXT_LOCALE_COOKIE, type Locale } from "@/i18n/locale";
import { CSRF_COOKIE, REFRESH_COOKIE, SESSION_COOKIE } from "@/lib/api/cookies";
import { logout, updateMe } from "@/lib/api/session";

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

/**
 * Sets the locale for this browser, and persists it against the account if
 * one is signed in.
 *
 * The cookie is the source of truth for every later render (resolveLocale's
 * first step); the account write is a best-effort sync for next time this
 * person signs in on a *different* device — see src/i18n/locale.ts's own
 * note on why that sync is not attempted the other way around.
 */
export async function setLocale(locale: string): Promise<void> {
  if (!isLocale(locale)) {
    return;
  }

  const jar = await cookies();
  jar.set(NEXT_LOCALE_COOKIE, locale satisfies Locale, { path: "/", maxAge: 60 * 60 * 24 * 365 });

  // Not signed in yet (e.g. the sign-in page's own switcher) is an expected
  // outcome here, not a failure worth surfacing.
  await updateMe({ locale });

  revalidatePath("/", "layout");
}
