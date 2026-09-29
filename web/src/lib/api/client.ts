/**
 * A thin, typed fetch wrapper over core-api.
 *
 * Request and response types come from `schema.d.ts`, which is generated from
 * `core-api/api/openapi.yaml`. Nothing here restates the contract, so a change
 * to the spec surfaces as a TypeScript error rather than as a runtime surprise.
 *
 * core-api owns identity, so this file will only ever forward credentials —
 * never mint or validate them.
 */
import type { components, operations } from "@/lib/api/schema";
import { coreApiBaseUrl } from "@/lib/api/config";

export type PingResult = components["schemas"]["PingResult"];
export type ApiError = components["schemas"]["Error"];

/** A successful call, or a reason it did not succeed. Never a thrown error. */
export type ApiResult<T> =
  { ok: true; data: T } | { ok: false; reason: "unavailable" | "error"; detail: string };

type FetchOptions = {
  /** Overrides the default base URL. Used by tests. */
  baseUrl?: string;
  fetchImpl?: typeof fetch;
  signal?: AbortSignal;
};

/**
 * Calls an endpoint and normalises every failure mode into an `ApiResult`.
 *
 * Callers get a value to render rather than an exception to catch, because the
 * product rule is that a screen must stay useful when data is missing — an
 * unreachable API is a state to display, not a crash.
 */
async function getJson<T>(path: string, options: FetchOptions = {}): Promise<ApiResult<T>> {
  const { baseUrl = coreApiBaseUrl(), fetchImpl = fetch, signal } = options;

  let response: Response;
  try {
    response = await fetchImpl(`${baseUrl}${path}`, {
      headers: { Accept: "application/json" },
      // A probe must never be served from a cache; Next 16 does not cache
      // fetches by default, but saying so keeps the intent explicit.
      cache: "no-store",
      signal,
    });
  } catch (error) {
    return {
      ok: false,
      reason: "unavailable",
      detail: error instanceof Error ? error.message : "core-api is unreachable",
    };
  }

  if (!response.ok) {
    const detail = await readErrorDetail(response);
    return {
      ok: false,
      reason: response.status >= 500 ? "unavailable" : "error",
      detail,
    };
  }

  return { ok: true, data: (await response.json()) as T };
}

async function readErrorDetail(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as Partial<ApiError>;
    if (typeof body.message === "string" && body.message.length > 0) {
      return body.message;
    }
  } catch {
    // A non-JSON error body is not itself an error worth surfacing.
  }

  return `core-api responded ${response.status}`;
}

/** Round-trip probe: proves web -> core-api -> Postgres end to end. */
export function getPing(options?: FetchOptions): Promise<ApiResult<PingResult>> {
  type Ok = operations["getPing"]["responses"][200]["content"]["application/json"];

  return getJson<Ok>("/api/v1/ping", options);
}
