/**
 * A thin, typed fetch wrapper over core-api.
 *
 * Request and response types come from `schema.d.ts`, which is generated from
 * `core-api/api/openapi.yaml`. Nothing here restates the contract, so a change
 * to the spec surfaces as a TypeScript error rather than as a runtime surprise.
 *
 * core-api owns identity, so this file will only ever forward credentials —
 * never mint or validate them. It takes the session cookie as a plain string
 * and stays free of `next/headers`, which keeps it testable in isolation;
 * `server.ts` is the layer that knows how to get one.
 */
import type { components, operations } from "@/lib/api/schema";
import { coreApiBaseUrl } from "@/lib/api/config";
import { CSRF_HEADER } from "@/lib/api/cookies";

export type PingResult = components["schemas"]["PingResult"];
export type ApiError = components["schemas"]["Error"];

/**
 * A successful call, or a reason it did not succeed. Never a thrown error.
 *
 * `unauthenticated` is separated from the other failures because it is the one
 * the caller can act on: it means redirect to sign-in, not show an error.
 */
export type ApiResult<T> =
  | { ok: true; data: T }
  | {
      ok: false;
      reason: "unauthenticated" | "forbidden" | "missing" | "unavailable" | "error";
      detail: string;
    };

export type ApiFailure = Extract<ApiResult<unknown>, { ok: false }>;

export type RequestOptions = {
  method?: "GET" | "POST" | "PATCH" | "DELETE";
  /** Serialised as JSON. Omitted for GET. */
  body?: unknown;
  /** Forwarded verbatim, so core-api sees the browser's session. */
  cookieHeader?: string;
  /** Echoed in the CSRF header; required by core-api for cookie mutations. */
  csrfToken?: string;
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
export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResult<T>> {
  const {
    method = "GET",
    body,
    cookieHeader,
    csrfToken,
    baseUrl = coreApiBaseUrl(),
    fetchImpl = fetch,
    signal,
  } = options;

  const headers = new Headers({ Accept: "application/json" });

  if (cookieHeader) {
    headers.set("Cookie", cookieHeader);
  }

  if (csrfToken) {
    headers.set(CSRF_HEADER, csrfToken);
  }

  if (body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  let response: Response;
  try {
    response = await fetchImpl(`${baseUrl}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      // Session-scoped data must never be served from a cache; Next 16 does
      // not cache fetches by default, but saying so keeps the intent explicit.
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
    return {
      ok: false,
      reason: failureReason(response.status),
      detail: await readErrorDetail(response),
    };
  }

  // 204 is a successful answer with nothing in it. Parsing it as JSON would
  // turn a sign-out into an error.
  if (response.status === 204) {
    return { ok: true, data: undefined as T };
  }

  return { ok: true, data: (await response.json()) as T };
}

function failureReason(status: number): ApiFailure["reason"] {
  switch (status) {
    case 401:
      return "unauthenticated";
    case 403:
      return "forbidden";
    case 404:
      return "missing";
    default:
      return status >= 500 ? "unavailable" : "error";
  }
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
export function getPing(options?: RequestOptions): Promise<ApiResult<PingResult>> {
  type Ok = operations["getPing"]["responses"][200]["content"]["application/json"];

  return apiRequest<Ok>("/api/v1/ping", options);
}
