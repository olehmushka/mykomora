"use server";

import { revalidatePath } from "next/cache";

import {
  archiveItem,
  attachItemTag,
  detachItemTag,
  moveItem,
  setItemStatus,
  type ItemStatus,
} from "@/lib/api/items";

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
