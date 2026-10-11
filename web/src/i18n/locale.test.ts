import { describe, expect, it } from "vitest";

import { isLocale, resolveLocale } from "@/i18n/locale";

describe("resolveLocale", () => {
  it("prefers the cookie over everything else", () => {
    expect(resolveLocale("uk", "en-US,en;q=0.9")).toBe("uk");
  });

  it("falls back to Accept-Language when there is no cookie", () => {
    expect(resolveLocale(undefined, "uk-UA,uk;q=0.9,en;q=0.5")).toBe("uk");
  });

  it("honours Accept-Language weights over header order", () => {
    expect(resolveLocale(undefined, "en;q=0.5,uk;q=0.9")).toBe("uk");
  });

  it("ignores an unsupported cookie value and falls through to the header", () => {
    expect(resolveLocale("fr", "uk;q=0.8")).toBe("uk");
  });

  it("falls back to English when nothing matches", () => {
    expect(resolveLocale(undefined, "fr-FR,de;q=0.5")).toBe("en");
  });

  it("falls back to English when there is no header at all", () => {
    expect(resolveLocale(undefined, undefined)).toBe("en");
  });
});

describe("isLocale", () => {
  it.each(["en", "uk"])("accepts %s", (value) => {
    expect(isLocale(value)).toBe(true);
  });

  it.each([undefined, "", "fr", "EN"])("rejects %j", (value) => {
    expect(isLocale(value)).toBe(false);
  });
});
