/**
 * Where core-api lives, as seen from the web server.
 *
 * Server-side only, and deliberately not a `NEXT_PUBLIC_` variable: the browser
 * talks to core-api through Caddy on the same origin, so the internal address
 * never needs to reach the client bundle.
 */
export function coreApiBaseUrl(): string {
  return process.env.CORE_API_URL ?? "http://localhost:8081";
}
