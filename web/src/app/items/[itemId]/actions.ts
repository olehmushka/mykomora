"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import {
  confirmItemAttachment,
  deleteItemAttachment,
  listItemAttachments,
  type ConfirmItemAttachmentRequest,
} from "@/lib/api/attachments";
import {
  archiveItem,
  attachItemTag,
  detachItemTag,
  duplicateItem,
  moveItem,
  setItemStatus,
  type ItemStatus,
} from "@/lib/api/items";
import {
  confirmItemPhoto,
  deleteItemPhoto,
  listItemPhotos,
  updateItemPhoto,
  type ConfirmItemPhotoRequest,
  type UpdateItemPhotoRequest,
} from "@/lib/api/photos";
import { flipItemSeason, undoItemBatch } from "@/lib/api/sizes";
import { presignUpload, type PresignUploadRequest } from "@/lib/api/uploads";

export type ActionState = { error?: string };

function optStr(form: FormData, key: string): string | undefined {
  const value = String(form.get(key) ?? "").trim();

  return value === "" ? undefined : value;
}

export async function move(itemId: string, _: ActionState, form: FormData): Promise<ActionState> {
  const parentItemId = optStr(form, "parentItemId");
  const locationId = optStr(form, "locationId");

  if (!parentItemId && !locationId) {
    return { error: "Choose a place or a container." };
  }

  const result = await moveItem(
    itemId,
    parentItemId
      ? { parentItemId }
      : {
          locationId,
          ...(optStr(form, "roomId") ? { roomId: optStr(form, "roomId") } : {}),
          ...(optStr(form, "spot") ? { spot: optStr(form, "spot") } : {}),
        },
  );

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(`/items/${itemId}`);

  return {};
}

export async function changeStatus(
  itemId: string,
  _: ActionState,
  form: FormData,
): Promise<ActionState> {
  const status = optStr(form, "status") as ItemStatus | undefined;
  if (!status) {
    return { error: "Choose a status." };
  }

  const result = await setItemStatus(itemId, status);

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(`/items/${itemId}`);

  return {};
}

export async function archive(itemId: string): Promise<void> {
  await archiveItem(itemId);
  revalidatePath(`/items/${itemId}`);
}

export async function duplicate(itemId: string): Promise<void> {
  const result = await duplicateItem(itemId, 1);
  if (!result.ok) {
    throw new Error(
      result.reason === "unavailable" ? "mykomora could not reach its API." : result.detail,
    );
  }

  revalidatePath("/items");

  const copy = result.data[0];
  redirect(copy ? `/items/${copy.id}` : "/items");
}

export async function attachTag(itemId: string, form: FormData): Promise<void> {
  const tagId = optStr(form, "tagId");
  if (tagId) {
    await attachItemTag(itemId, tagId);
    revalidatePath(`/items/${itemId}`);
  }
}

export async function detachTag(itemId: string, form: FormData): Promise<void> {
  const tagId = optStr(form, "tagId");
  if (tagId) {
    await detachItemTag(itemId, tagId);
    revalidatePath(`/items/${itemId}`);
  }
}

// --- photos --------------------------------------------------------------
//
// These are called directly from client components (photo-gallery.tsx,
// attachments-section.tsx) as plain async functions, not via <form
// action=.../useActionState — the upload flow needs client-side control
// (resize, two presigned PUTs, then confirm) that FormData can't carry.
// They stay "use server" wrappers over lib/api/* rather than exporting
// those directly, because only a function in a "use server" file can be
// called this way from a client bundle at all.

export async function presignUploadAction(body: PresignUploadRequest) {
  return presignUpload(body);
}

export async function refreshPhotos(itemId: string) {
  return listItemPhotos(itemId);
}

export async function confirmPhoto(itemId: string, body: ConfirmItemPhotoRequest) {
  const result = await confirmItemPhoto(itemId, body);
  if (result.ok) revalidatePath(`/items/${itemId}`);

  return result;
}

export async function updatePhoto(itemId: string, photoId: string, body: UpdateItemPhotoRequest) {
  const result = await updateItemPhoto(itemId, photoId, body);
  if (result.ok) revalidatePath(`/items/${itemId}`);

  return result;
}

export async function deletePhoto(itemId: string, photoId: string) {
  const result = await deleteItemPhoto(itemId, photoId);
  if (result.ok) revalidatePath(`/items/${itemId}`);

  return result;
}

// --- attachments -----------------------------------------------------------

export async function refreshAttachments(itemId: string) {
  return listItemAttachments(itemId);
}

export async function confirmAttachment(itemId: string, body: ConfirmItemAttachmentRequest) {
  const result = await confirmItemAttachment(itemId, body);
  if (result.ok) revalidatePath(`/items/${itemId}`);

  return result;
}

export async function deleteAttachment(itemId: string, attachmentId: string) {
  const result = await deleteItemAttachment(itemId, attachmentId);
  if (result.ok) revalidatePath(`/items/${itemId}`);

  return result;
}

export type FlipState = { error?: string; batchId?: string; changed?: number; skipped?: number };

const UNAVAILABLE = "mykomora could not reach its API. Try again in a moment.";

/** Pack (in_use → stored) or unpack (stored → in_use) a container's contents. */
export async function flipSeason(itemId: string, direction: "pack" | "unpack"): Promise<FlipState> {
  const result = await flipItemSeason(itemId, direction);
  if (!result.ok) {
    return { error: result.reason === "unavailable" ? UNAVAILABLE : result.detail };
  }

  revalidatePath(`/items/${itemId}`);

  return {
    batchId: result.data.batchId,
    changed: result.data.changed,
    skipped: result.data.skipped,
  };
}

export async function undoFlip(itemId: string, batchId: string): Promise<ActionState> {
  const result = await undoItemBatch(batchId);
  if (!result.ok) {
    return { error: result.reason === "unavailable" ? UNAVAILABLE : result.detail };
  }

  revalidatePath(`/items/${itemId}`);

  return {};
}
