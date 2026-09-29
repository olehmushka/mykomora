import { describe, expect, it } from "vitest";

import { apiRequest, getPing } from "@/lib/api/client";
import { CSRF_HEADER } from "@/lib/api/cookies";

const baseUrl = "http://core-api.test";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("getPing", () => {
  it("returns the payload when core-api answers", async () => {
    const now = "2026-09-29T12:00:00Z";
    const result = await getPing({
      baseUrl,
      fetchImpl: async () => jsonResponse(200, { now, database: "ok" }),
    });

    expect(result).toEqual({ ok: true, data: { now, database: "ok" } });
  });

  it("requests the path from the contract", async () => {
    const seen: string[] = [];
    await getPing({
      baseUrl,
      fetchImpl: async (input) => {
        seen.push(String(input));
        return jsonResponse(200, { now: "2026-09-29T12:00:00Z", database: "ok" });
      },
    });

    expect(seen).toEqual([`${baseUrl}/api/v1/ping`]);
  });

  // The product rule is that a screen stays useful when data is missing, so the
  // client must hand back a value to render rather than throw at the caller.
  it("reports an unreachable API without throwing", async () => {
    const result = await getPing({
      baseUrl,
      fetchImpl: async () => {
        throw new TypeError("fetch failed");
      },
    });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe("unavailable");
      expect(result.detail).toContain("fetch failed");
    }
  });

  it("surfaces the message from a 503 body", async () => {
    const result = await getPing({
      baseUrl,
      fetchImpl: async () =>
        jsonResponse(503, {
          code: "database_unavailable",
          message: "the database did not answer",
        }),
    });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe("unavailable");
      expect(result.detail).toBe("the database did not answer");
    }
  });

  it("falls back to the status code when the error body is not JSON", async () => {
    const result = await getPing({
      baseUrl,
      fetchImpl: async () => new Response("<html>502</html>", { status: 502 }),
    });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.detail).toBe("core-api responded 502");
    }
  });
});

describe("apiRequest", () => {
  // core-api owns identity; this client's whole job on the way out is to hand
  // the browser's credentials over unchanged.
  it("forwards the session cookies", async () => {
    let seen: Headers | undefined;

    await apiRequest("/api/v1/me", {
      baseUrl,
      cookieHeader: "mk_session=a-token; mk_csrf=a-csrf",
      fetchImpl: async (_input, init) => {
        seen = new Headers(init?.headers);
        return jsonResponse(200, {});
      },
    });

    expect(seen?.get("cookie")).toBe("mk_session=a-token; mk_csrf=a-csrf");
  });

  it("sends the CSRF token on a mutation", async () => {
    let seen: Headers | undefined;

    await apiRequest("/api/v1/family", {
      baseUrl,
      method: "PATCH",
      body: { name: "Мушка" },
      csrfToken: "a-csrf",
      fetchImpl: async (_input, init) => {
        seen = new Headers(init?.headers);
        return jsonResponse(200, {});
      },
    });

    expect(seen?.get(CSRF_HEADER)).toBe("a-csrf");
    expect(seen?.get("content-type")).toBe("application/json");
  });

  it("sends a JSON body only when there is one", async () => {
    let body: BodyInit | null | undefined;

    await apiRequest("/api/v1/people", {
      baseUrl,
      fetchImpl: async (_input, init) => {
        body = init?.body;
        return jsonResponse(200, []);
      },
    });

    expect(body).toBeUndefined();
  });

  // Sign-out and archive answer 204. Parsing that as JSON would turn a
  // success into an error the user cannot act on.
  it("treats 204 as a success with no payload", async () => {
    const result = await apiRequest("/api/v1/auth/logout", {
      baseUrl,
      method: "POST",
      fetchImpl: async () => new Response(null, { status: 204 }),
    });

    expect(result.ok).toBe(true);
  });

  // The caller can act on a 401 — redirect to sign-in — but only if it is
  // distinguishable from core-api being unwell.
  it.each([
    [401, "unauthenticated"],
    [403, "forbidden"],
    [404, "missing"],
    [400, "error"],
    [500, "unavailable"],
  ])("maps %i onto the %s reason", async (status, reason) => {
    const result = await apiRequest("/api/v1/family", {
      baseUrl,
      fetchImpl: async () => jsonResponse(status, { code: "x", message: "nope" }),
    });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe(reason);
    }
  });
});
