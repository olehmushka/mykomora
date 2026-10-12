import { describe, expect, it } from "vitest";

import en from "../../messages/en.json";
import uk from "../../messages/uk.json";

function keys(value: unknown, prefix = ""): string[] {
  if (typeof value !== "object" || value === null) {
    return [prefix];
  }

  return Object.entries(value).flatMap(([key, child]) =>
    keys(child, prefix === "" ? key : `${prefix}.${key}`),
  );
}

describe("message catalogues", () => {
  it("define the same keys in en and uk", () => {
    expect(keys(uk).sort()).toEqual(keys(en).sort());
  });

  it("leave no uk message empty", () => {
    const empty = keys(uk).filter((key) => {
      const value = key.split(".").reduce<unknown>((node, part) => (node as never)[part], uk);

      return typeof value !== "string" || value.trim() === "";
    });

    expect(empty).toEqual([]);
  });
});
