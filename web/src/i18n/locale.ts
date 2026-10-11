/**
 * Locale resolution, shared between next-intl's request config and the
 * core-api fetch wrapper.
 *
 * SPEC.md's resolution order also includes GeoIP (an IP geolocated against
 * MaxMind's GeoLite2-Country database). That step is deliberately dropped —
 * it would mean an account, a license key and a redistribution/update story
 * for a database file, for a single-family app with no public sign-up. The
 * order here is therefore: the `NEXT_LOCALE` cookie, then the browser's own
 * Accept-Language, then English. See docs/MILESTONES.md's M2 delivery note.
 *
 * A signed-in user's *stored* preference (core-api's `users.locale`) is not
 * consulted on every render — that would mean an extra `/me` round trip on
 * every page load. It is written into this cookie instead, whenever it is
 * learned: when the locale switcher is used while signed in. A fresh sign-in
 * on a cookie-less device therefore sees this chain's guess until the
 * switcher is used there too — an accepted gap for a single-family app.
 */
export const locales = ["en", "uk"] as const;

export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "en";

export const NEXT_LOCALE_COOKIE = "NEXT_LOCALE";

/** Resolves a locale from the cookie, then Accept-Language, then English. */
export function resolveLocale(
  cookieValue: string | undefined,
  acceptLanguage: string | undefined,
): Locale {
  if (isLocale(cookieValue)) {
    return cookieValue;
  }

  const fromHeader = pickFromAcceptLanguage(acceptLanguage);
  if (fromHeader) {
    return fromHeader;
  }

  return defaultLocale;
}

export function isLocale(value: string | undefined): value is Locale {
  return value !== undefined && (locales as readonly string[]).includes(value);
}

/**
 * Picks the first supported locale an Accept-Language header names, honouring
 * its `;q=` weights (highest first, original order preserved on ties).
 *
 * core-api's `internal/locale` package has the same small parser for the same
 * reason: the header format needs no dependency to read when there are only
 * two locales to match against.
 */
function pickFromAcceptLanguage(header: string | undefined): Locale | null {
  if (!header) {
    return null;
  }

  const candidates = header
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const [rawTag, ...params] = part.split(";");
      const tag = (rawTag ?? "").trim().split("-")[0]?.toLowerCase();
      const weightParam = params.map((p) => p.trim()).find((p) => p.startsWith("q="));
      const weight = weightParam ? Number.parseFloat(weightParam.slice(2)) : 1;

      return { tag, weight: Number.isNaN(weight) ? 0 : weight };
    });

  candidates.sort((a, b) => b.weight - a.weight);

  for (const candidate of candidates) {
    if (isLocale(candidate.tag)) {
      return candidate.tag;
    }
  }

  return null;
}
