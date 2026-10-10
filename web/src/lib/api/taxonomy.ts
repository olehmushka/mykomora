import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Category = components["schemas"]["Category"];
export type TraitDef = components["schemas"]["TraitDef"];
export type Taxonomy = components["schemas"]["Taxonomy"];
export type CreateCategoryRequest = components["schemas"]["CreateCategoryRequest"];
export type CreateTraitDefRequest = components["schemas"]["CreateTraitDefRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

/** The localised category tree. Resolved from the request's own Accept-Language. */
export function getTaxonomy() {
  return callApi<Ok<"getTaxonomy", 200>>("/api/v1/taxonomy");
}

export function createCategory(body: CreateCategoryRequest) {
  return callApi<Ok<"createCategory", 201>>("/api/v1/categories", { method: "POST", body });
}

export function archiveCategory(categoryId: string) {
  return callApi<void>(`/api/v1/categories/${encodeURIComponent(categoryId)}`, {
    method: "DELETE",
  });
}

export function createTraitDef(categoryId: string, body: CreateTraitDefRequest) {
  return callApi<Ok<"createTraitDef", 201>>(
    `/api/v1/categories/${encodeURIComponent(categoryId)}/trait-defs`,
    {
      method: "POST",
      body,
    },
  );
}

export function archiveTraitDef(traitDefId: string) {
  return callApi<void>(`/api/v1/trait-defs/${encodeURIComponent(traitDefId)}`, {
    method: "DELETE",
  });
}
