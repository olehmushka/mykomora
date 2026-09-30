import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Person = components["schemas"]["Person"];
export type CreatePersonRequest = components["schemas"]["CreatePersonRequest"];
export type UpdatePersonRequest = components["schemas"]["UpdatePersonRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listPeople(includeArchived = false) {
  const query = includeArchived ? "?includeArchived=true" : "";

  return callApi<Ok<"listPeople", 200>>(`/api/v1/people${query}`);
}

export function createPerson(body: CreatePersonRequest) {
  return callApi<Ok<"createPerson", 201>>("/api/v1/people", { method: "POST", body });
}

export function updatePerson(personId: string, body: UpdatePersonRequest) {
  return callApi<Ok<"updatePerson", 200>>(`/api/v1/people/${encodeURIComponent(personId)}`, {
    method: "PATCH",
    body,
  });
}

export function archivePerson(personId: string) {
  return callApi<void>(`/api/v1/people/${encodeURIComponent(personId)}`, { method: "DELETE" });
}
