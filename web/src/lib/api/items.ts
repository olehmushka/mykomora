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

export type ListItemsParams = {
  locationId?: string;
  roomId?: string;
  categoryId?: string;
  includeArchived?: boolean;
};

function itemsQuery(params: ListItemsParams): string {
  const search = new URLSearchParams();

  if (params.locationId) search.set("locationId", params.locationId);
  if (params.roomId) search.set("roomId", params.roomId);
  if (params.categoryId) search.set("categoryId", params.categoryId);
  if (params.includeArchived) search.set("includeArchived", "true");

  const query = search.toString();

  return query === "" ? "" : `?${query}`;
}

export function listItems(params: ListItemsParams = {}) {
  return callApi<Ok<"listItems", 200>>(`/api/v1/items${itemsQuery(params)}`);
}

export function createItem(body: CreateItemRequest) {
  return callApi<Ok<"createItem", 201>>("/api/v1/items", { method: "POST", body });
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
