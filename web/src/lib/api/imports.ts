import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type ImportInspection = components["schemas"]["ImportInspection"];
export type ImportRequest = components["schemas"]["ImportRequest"];
export type ImportReport = components["schemas"]["ImportReport"];
export type ImportResult = components["schemas"]["ImportResult"];
export type ImportPresign = components["schemas"]["PresignUploadResult"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function presignImport(contentType: string, contentLength: number) {
  return callApi<Ok<"presignImport", 200>>("/api/v1/imports/presign", {
    method: "POST",
    body: { contentType, contentLength },
  });
}

export function inspectImport(storageKey: string) {
  return callApi<Ok<"inspectImport", 200>>("/api/v1/imports/inspect", {
    method: "POST",
    body: { storageKey },
  });
}

export function dryRunImport(body: ImportRequest) {
  return callApi<Ok<"dryRunImport", 200>>("/api/v1/imports/dry-run", { method: "POST", body });
}

export function commitImport(body: ImportRequest) {
  return callApi<Ok<"commitImport", 201>>("/api/v1/imports/commit", { method: "POST", body });
}
