import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Item = components["schemas"]["Item"];
export type ItemStatus = components["schemas"]["ItemStatus"];
export type ItemCondition = components["schemas"]["ItemCondition"];
export type ItemSeason = components["schemas"]["ItemSeason"];
export type SizeSystem = components["schemas"]["SizeSystem"];
export type ItemEvent = components["schemas"]["ItemEvent"];
export type CreateItemRequest = components["schemas"]["CreateItemRequest"];
export type UpdateItemRequest = components["schemas"]["UpdateItemRequest"];
export type MoveItemRequest = components["schemas"]["MoveItemRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export type ItemPage = components["schemas"]["ItemPage"];
export type ItemHit = components["schemas"]["ItemHit"];
export type ItemPlacement = components["schemas"]["ItemPlacement"];
export type SimilarItem = components["schemas"]["SimilarItem"];
export type Dashboard = components["schemas"]["Dashboard"];
export type ItemSort = NonNullable<
  NonNullable<operations["listItems"]["parameters"]["query"]>["sort"]
>;

/**
 * Everything GET /items accepts. All strings are passed through as typed by
 * the URL: core-api validates them, and a bad one comes back as a 400 the
 * page can show rather than something this layer second-guesses.
 */
export type ListItemsParams = {
  q?: string;
  categoryId?: string;
  locationId?: string;
  roomId?: string;
  containerId?: string;
  personId?: string;
  tagId?: string;
  status?: string;
  season?: string;
  condition?: string;
  /** JSON object of filterable trait keys to required values. */
  traits?: string;
  expiresFrom?: string;
  expiresTo?: string;
  warrantyFrom?: string;
  warrantyTo?: string;
  priceMin?: string;
  priceMax?: string;
  sort?: string;
  cursor?: string;
  limit?: number;
  includeArchived?: boolean;
};

export function itemsQuery(params: ListItemsParams): string {
  const search = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "" || value === false) continue;

    search.set(key, String(value));
  }

  const query = search.toString();

  return query === "" ? "" : `?${query}`;
}

export function listItems(params: ListItemsParams = {}) {
  return callApi<Ok<"listItems", 200>>(`/api/v1/items${itemsQuery(params)}`);
}

export function findSimilarItems(name: string) {
  return callApi<Ok<"findSimilarItems", 200>>(
    `/api/v1/items/similar?name=${encodeURIComponent(name)}`,
  );
}

export function getDashboard() {
  return callApi<Ok<"getDashboard", 200>>("/api/v1/dashboard");
}

export function createItem(body: CreateItemRequest) {
  return callApi<Ok<"createItem", 201>>("/api/v1/items", { method: "POST", body });
}

export function createItemsBatch(items: CreateItemRequest[]) {
  return callApi<Ok<"createItemsBatch", 201>>("/api/v1/items/batch", {
    method: "POST",
    body: { items },
  });
}

export function duplicateItem(itemId: string, count = 1) {
  return callApi<Ok<"duplicateItem", 201>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/duplicate`,
    { method: "POST", body: { count } },
  );
}

export function getItem(itemId: string) {
  return callApi<Ok<"getItem", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}`);
}

export function updateItem(itemId: string, body: UpdateItemRequest) {
  return callApi<Ok<"updateItem", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}`, {
    method: "PATCH",
    body,
  });
}

export function archiveItem(itemId: string) {
  return callApi<void>(`/api/v1/items/${encodeURIComponent(itemId)}`, { method: "DELETE" });
}

export function moveItem(itemId: string, body: MoveItemRequest) {
  return callApi<Ok<"moveItem", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/move`, {
    method: "POST",
    body,
  });
}

export function setItemStatus(itemId: string, status: ItemStatus) {
  return callApi<Ok<"setItemStatus", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/status`, {
    method: "POST",
    body: { status },
  });
}

export function listItemContents(itemId: string) {
  return callApi<Ok<"listItemContents", 200>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/contents`,
  );
}

export function listItemEvents(itemId: string) {
  return callApi<Ok<"listItemEvents", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/events`);
}

export function listItemTags(itemId: string) {
  return callApi<Ok<"listItemTags", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/tags`);
}

export function attachItemTag(itemId: string, tagId: string) {
  return callApi<void>(`/api/v1/items/${encodeURIComponent(itemId)}/tags`, {
    method: "POST",
    body: { tagId },
  });
}

export function detachItemTag(itemId: string, tagId: string) {
  return callApi<void>(
    `/api/v1/items/${encodeURIComponent(itemId)}/tags/${encodeURIComponent(tagId)}`,
    { method: "DELETE" },
  );
}
