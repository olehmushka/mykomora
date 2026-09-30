import { describe, expect, it } from "vitest";

import { mergeCookieHeader } from "@/proxy";

describe("mergeCookieHeader", () => {
  it("replaces the rotated cookies and keeps the rest", () => {
    const merged = mergeCookieHeader("mk_session=old; mk_refresh=old-refresh; theme=dark", [
      "mk_session=new; Path=/; HttpOnly; SameSite=Lax",
      "mk_refresh=new-refresh; Path=/; HttpOnly; SameSite=Lax",
    ]);

    expect(merged).toBe("mk_session=new; mk_refresh=new-refresh; theme=dark");
  });

  it("adds a cookie the request did not have", () => {
    const merged = mergeCookieHeader("mk_refresh=old-refresh", [
      "mk_session=new; Path=/",
      "mk_csrf=a-csrf; Path=/",
    ]);

    expect(merged).toBe("mk_refresh=old-refresh; mk_session=new; mk_csrf=a-csrf");
  });

  it("works when the request carried no cookies at all", () => {
    expect(mergeCookieHeader(null, ["mk_session=new; Path=/"])).toBe("mk_session=new");
  });

  // Tokens are base64url and can contain "=" padding characters, so splitting
  // on every "=" would truncate them.
  it("keeps a value containing an equals sign intact", () => {
    const merged = mergeCookieHeader(null, ["mk_refresh=abc==; Path=/"]);

    expect(merged).toBe("mk_refresh=abc==");
  });

  it("ignores a malformed Set-Cookie rather than writing a broken header", () => {
    expect(mergeCookieHeader("mk_session=old", ["; Path=/"])).toBe("mk_session=old");
  });
});
