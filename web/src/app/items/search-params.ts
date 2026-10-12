import type { ListItemsParams } from "@/lib/api/items";
import type { TraitDef } from "@/lib/api/taxonomy";

type RawSearchParams = Record<string, string | string[] | undefined>;

/** The URL parameters the items page understands, besides `trait__*`. */
const FILTER_KEYS = [
  "q",
  "categoryId",
  "locationId",
  "roomId",
  "personId",
  "tagId",
  "status",
  "season",
  "condition",
  "expiresFrom",
  "expiresTo",
  "warrantyFrom",
  "warrantyTo",
  "priceMin",
  "priceMax",
  "sort",
  "cursor",
  "view",
] as const;

export const TRAIT_PREFIX = "trait__";

export type FilterValues = Partial<Record<(typeof FILTER_KEYS)[number], string>> &
  Record<string, string | undefined>;

function first(value: string | string[] | undefined): string | undefined {
  const one = Array.isArray(value) ? value[0] : value;

  return one === undefined || one.trim() === "" ? undefined : one.trim();
}

/** Picks the recognised, non-empty parameters out of Next's `searchParams`. */
export function readFilterValues(sp: RawSearchParams): FilterValues {
  const out: FilterValues = {};

  for (const [key, raw] of Object.entries(sp)) {
    const known = (FILTER_KEYS as readonly string[]).includes(key) || key.startsWith(TRAIT_PREFIX);
    const value = known ? first(raw) : undefined;

    if (value !== undefined) out[key] = value;
  }

  return out;
}

/**
 * The `traits` parameter core-api expects: a JSON object of the selected
 * category's filterable fields. Values are coerced to the JSON type each
 * field's validator wants; anything that does not fit is dropped rather than
 * sent to be rejected.
 */
export function traitsParam(values: FilterValues, defs: TraitDef[]): string | undefined {
  const wanted: Record<string, string | number | boolean> = {};

  for (const def of defs) {
    if (!def.filterable) continue;

    const raw = values[`${TRAIT_PREFIX}${def.key}`];
    if (raw === undefined) continue;

    if (def.dataType === "bool") {
      if (raw === "true") wanted[def.key] = true;
    } else if (def.dataType === "number" || def.dataType === "integer") {
      const n = Number(raw);
      if (Number.isFinite(n)) wanted[def.key] = n;
    } else {
      wanted[def.key] = raw;
    }
  }

  return Object.keys(wanted).length === 0 ? undefined : JSON.stringify(wanted);
}

export function toApiParams(values: FilterValues, defs: TraitDef[]): ListItemsParams {
  return {
    q: values.q,
    categoryId: values.categoryId,
    locationId: values.locationId,
    roomId: values.roomId,
    personId: values.personId,
    tagId: values.tagId,
    status: values.status,
    season: values.season,
    condition: values.condition,
    traits: traitsParam(values, defs),
    expiresFrom: values.expiresFrom,
    expiresTo: values.expiresTo,
    warrantyFrom: values.warrantyFrom,
    warrantyTo: values.warrantyTo,
    priceMin: values.priceMin,
    priceMax: values.priceMax,
    sort: values.sort,
    cursor: values.cursor,
  };
}

/** `/items?…` for the current filters with some parameters replaced or removed. */
export function hrefWith(
  values: FilterValues,
  changes: Record<string, string | undefined>,
): string {
  const next = { ...values, ...changes };
  const search = new URLSearchParams();

  for (const [key, value] of Object.entries(next)) {
    if (value !== undefined && value !== "") search.set(key, value);
  }

  const query = search.toString();

  return query === "" ? "/items" : `/items?${query}`;
}

/**
 * True when anything beyond view/sort/cursor narrows the list, which is when
 * "no results" is about the filters rather than about an empty inventory.
 */
export function hasActiveFilters(values: FilterValues): boolean {
  return Object.keys(values).some((key) => !["view", "sort", "cursor"].includes(key));
}
