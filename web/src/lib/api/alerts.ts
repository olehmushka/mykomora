import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Alert = components["schemas"]["Alert"];
export type AlertKind = components["schemas"]["AlertKind"];
export type AlertList = components["schemas"]["AlertList"];
export type AlertDismissal = components["schemas"]["AlertDismissal"];
export type ShoppingList = components["schemas"]["ShoppingList"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listAlerts() {
  return callApi<Ok<"listAlerts", 200>>("/api/v1/alerts");
}

export function dismissAlert(itemId: string, kind: AlertKind, snoozeDays: number) {
  return callApi<Ok<"dismissAlert", 200>>("/api/v1/alerts/dismissals", {
    method: "POST",
    body: { itemId, kind, snoozeDays },
  });
}

export function undoAlertDismissal(dismissalId: string) {
  return callApi<void>(`/api/v1/alerts/dismissals/${encodeURIComponent(dismissalId)}`, {
    method: "DELETE",
  });
}

export function getShoppingList() {
  return callApi<Ok<"getShoppingList", 200>>("/api/v1/shopping-list");
}
