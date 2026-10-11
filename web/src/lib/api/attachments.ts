import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type ItemAttachment = components["schemas"]["ItemAttachment"];
export type AttachmentKind = components["schemas"]["AttachmentKind"];
export type ConfirmItemAttachmentRequest = components["schemas"]["ConfirmItemAttachmentRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listItemAttachments(itemId: string) {
  return callApi<Ok<"listItemAttachments", 200>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/attachments`,
  );
}

export function confirmItemAttachment(itemId: string, body: ConfirmItemAttachmentRequest) {
  return callApi<Ok<"confirmItemAttachment", 201>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/attachments`,
    { method: "POST", body },
  );
}

export function deleteItemAttachment(itemId: string, attachmentId: string) {
  return callApi<void>(
    `/api/v1/items/${encodeURIComponent(itemId)}/attachments/${encodeURIComponent(attachmentId)}`,
    { method: "DELETE" },
  );
}
