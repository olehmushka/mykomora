"use server";

import { revalidatePath } from "next/cache";

import { bulkSetItemStatus, recordPersonSize, undoItemBatch } from "@/lib/api/sizes";
import type { SizeSystem, SizeType } from "@/lib/api/sizes";
import { apiError, errorText } from "@/lib/api-error";

export type ActionState = { error?: string };

export type BatchState = { error?: string; batchId?: string; changed?: number };

export async function recordSize(
  personId: string,
  _: ActionState,
  form: FormData,
): Promise<ActionState> {
  const sizeValue = String(form.get("sizeValue") ?? "").trim();
  if (sizeValue === "") {
    return { error: await errorText("sizeRequired") };
  }

  const result = await recordPersonSize(personId, {
    sizeType: String(form.get("sizeType")) as SizeType,
    sizeSystem: String(form.get("sizeSystem")) as SizeSystem,
    sizeValue,
  });

  if (!result.ok) {
    return { error: await apiError(result) };
  }

  revalidatePath(`/people/${personId}`, "layout");

  return {};
}

/** The "outgrown" sweep: everything ticked goes to to_give_away, undoable as one. */
export async function markOutgrown(personId: string, itemIds: string[]): Promise<BatchState> {
  if (itemIds.length === 0) {
    return { error: await errorText("tickItem") };
  }

  const result = await bulkSetItemStatus(itemIds, "to_give_away");
  if (!result.ok) {
    return { error: await apiError(result) };
  }

  revalidatePath(`/people/${personId}/fits`);

  return { batchId: result.data.batchId, changed: result.data.changed };
}

export async function undoOutgrown(personId: string, batchId: string): Promise<ActionState> {
  const result = await undoItemBatch(batchId);
  if (!result.ok) {
    return { error: await apiError(result) };
  }

  revalidatePath(`/people/${personId}/fits`);

  return {};
}
