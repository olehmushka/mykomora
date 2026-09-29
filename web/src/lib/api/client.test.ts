import { describe, expect, it } from "vitest";

import { getPing } from "@/lib/api/client";

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
