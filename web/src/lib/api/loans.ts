import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Loan = components["schemas"]["Loan"];
export type LendItemRequest = components["schemas"]["LendItemRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function lendItem(itemId: string, body: LendItemRequest) {
  return callApi<Ok<"lendItem", 201>>(`/api/v1/items/${encodeURIComponent(itemId)}/lend`, {
    method: "POST",
    body,
  });
}

export function returnItem(itemId: string) {
  return callApi<Ok<"returnItem", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/return`, {
    method: "POST",
  });
}

export function listItemLoans(itemId: string) {
  return callApi<Ok<"listItemLoans", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/loans`);
}

export function listOpenLoans() {
  return callApi<Ok<"listOpenLoans", 200>>("/api/v1/loans");
}
