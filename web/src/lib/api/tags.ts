import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Tag = components["schemas"]["Tag"];
export type CreateTagRequest = components["schemas"]["CreateTagRequest"];
export type UpdateTagRequest = components["schemas"]["UpdateTagRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listTags() {
  return callApi<Ok<"listTags", 200>>("/api/v1/tags");
}

export function createTag(body: CreateTagRequest) {
  return callApi<Ok<"createTag", 201>>("/api/v1/tags", { method: "POST", body });
}

export function updateTag(tagId: string, body: UpdateTagRequest) {
  return callApi<Ok<"updateTag", 200>>(`/api/v1/tags/${encodeURIComponent(tagId)}`, {
    method: "PATCH",
    body,
  });
}

export function deleteTag(tagId: string) {
  return callApi<void>(`/api/v1/tags/${encodeURIComponent(tagId)}`, { method: "DELETE" });
}
