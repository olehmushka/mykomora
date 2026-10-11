import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type ItemPhoto = components["schemas"]["ItemPhoto"];
export type ConfirmItemPhotoRequest = components["schemas"]["ConfirmItemPhotoRequest"];
export type UpdateItemPhotoRequest = components["schemas"]["UpdateItemPhotoRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listItemPhotos(itemId: string) {
  return callApi<Ok<"listItemPhotos", 200>>(`/api/v1/items/${encodeURIComponent(itemId)}/photos`);
}

export function confirmItemPhoto(itemId: string, body: ConfirmItemPhotoRequest) {
  return callApi<Ok<"confirmItemPhoto", 201>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/photos`,
    {
      method: "POST",
      body,
    },
  );
}

export function updateItemPhoto(itemId: string, photoId: string, body: UpdateItemPhotoRequest) {
  return callApi<Ok<"updateItemPhoto", 200>>(
    `/api/v1/items/${encodeURIComponent(itemId)}/photos/${encodeURIComponent(photoId)}`,
    { method: "PATCH", body },
  );
}

export function deleteItemPhoto(itemId: string, photoId: string) {
  return callApi<void>(
    `/api/v1/items/${encodeURIComponent(itemId)}/photos/${encodeURIComponent(photoId)}`,
    {
      method: "DELETE",
    },
  );
}
