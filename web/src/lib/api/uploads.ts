import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type UploadPurpose = components["schemas"]["UploadPurpose"];
export type PresignUploadRequest = components["schemas"]["PresignUploadRequest"];
export type PresignUploadResult = components["schemas"]["PresignUploadResult"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function presignUpload(body: PresignUploadRequest) {
  return callApi<Ok<"presignUpload", 200>>("/api/v1/uploads/presign", { method: "POST", body });
}
