import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";
import type { ItemStatus } from "@/lib/api/items";

export type PersonSize = components["schemas"]["PersonSize"];
export type SizeType = components["schemas"]["SizeType"];
export type SizeSystem = components["schemas"]["SizeSystem"];
export type RecordPersonSizeRequest = components["schemas"]["RecordPersonSizeRequest"];
export type PersonFits = components["schemas"]["PersonFits"];
export type FitSet = components["schemas"]["FitSet"];
export type BatchResult = components["schemas"]["BatchResult"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listPersonSizes(personId: string) {
  return callApi<Ok<"listPersonSizes", 200>>(
    `/api/v1/people/${encodeURIComponent(personId)}/sizes`,
  );
}

export function recordPersonSize(personId: string, body: RecordPersonSizeRequest) {
  return callApi<Ok<"recordPersonSize", 201>>(
    `/api/v1/people/${encodeURIComponent(personId)}/sizes`,
    { method: "POST", body },
  );
}

export function getPersonFits(personId: string, offset: 0 | 1) {
  return callApi<Ok<"getPersonFits", 200>>(
    `/api/v1/people/${encodeURIComponent(personId)}/fits?offset=${offset}`,
  );
}

export function bulkSetItemStatus(itemIds: string[], status: ItemStatus) {
  return callApi<Ok<"bulkSetItemStatus", 200>>("/api/v1/items/bulk-status", {
    method: "POST",
    body: { itemIds, status },
  });
}

export function flipItemSeason(itemId: string, direction: "pack" | "unpack") {
  return callApi<Ok<"flipItemSeason", 200>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/season-flip`,
    { method: "POST", body: { direction } },
  );
}

export function undoItemBatch(batchId: string) {
  return callApi<Ok<"undoItemBatch", 200>>(
    `/api/v1/item-batches/${encodeURIComponent(batchId)}/undo`,
    { method: "POST" },
  );
}
