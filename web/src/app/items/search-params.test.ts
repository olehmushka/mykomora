import { describe, expect, it } from "vitest";

import {
  hasActiveFilters,
  hrefWith,
  readFilterValues,
  toApiParams,
  traitsParam,
} from "@/app/items/search-params";
import type { TraitDef } from "@/lib/api/taxonomy";

function def(key: string, dataType: TraitDef["dataType"], filterable = true): TraitDef {
  return {
    id: key,
    categoryId: "c",
    key,
    dataType,
    label: key,
    required: false,
    filterable,
    sortOrder: 0,
    isBuiltin: true,
  };
}

describe("readFilterValues", () => {
  it("keeps recognised, non-empty parameters and drops the rest", () => {
    const values = readFilterValues({
      q: " зимові чоботи ",
      status: "",
      unknown: "x",
      trait__shoe_type: "boots",
      roomId: ["a", "b"],
    });

    expect(values).toEqual({ q: "зимові чоботи", trait__shoe_type: "boots", roomId: "a" });
  });
});

describe("traitsParam", () => {
  const defs = [
    def("shoe_type", "enum"),
    def("waterproof", "bool"),
    def("heel_cm", "number"),
    def("secret", "text", false),
  ];

  it("coerces each value to its field's JSON type and ignores non-filterable fields", () => {
    const json = traitsParam(
      {
        trait__shoe_type: "boots",
        trait__waterproof: "true",
        trait__heel_cm: "3.5",
        trait__secret: "x",
      },
      defs,
    );

    expect(JSON.parse(json ?? "{}")).toEqual({
      shoe_type: "boots",
      waterproof: true,
      heel_cm: 3.5,
    });
  });

  it("returns nothing when no trait is set or a number does not parse", () => {
    expect(traitsParam({}, defs)).toBeUndefined();
    expect(traitsParam({ trait__heel_cm: "abc" }, defs)).toBeUndefined();
  });
});

describe("hrefWith", () => {
  it("replaces and removes parameters without touching the rest", () => {
    const values = readFilterValues({ q: "box", cursor: "abc", view: "grid" });

    expect(hrefWith(values, { cursor: undefined })).toBe("/items?q=box&view=grid");
    expect(hrefWith({}, {})).toBe("/items");
  });
});

describe("hasActiveFilters", () => {
  it("ignores view, sort and cursor", () => {
    expect(hasActiveFilters({ view: "grid", sort: "name", cursor: "x" })).toBe(false);
    expect(hasActiveFilters({ q: "x" })).toBe(true);
  });
});

describe("toApiParams", () => {
  it("passes the plain filters through", () => {
    expect(toApiParams({ q: "x", roomId: "r" }, []).roomId).toBe("r");
  });
});
