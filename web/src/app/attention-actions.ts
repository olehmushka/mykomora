"use server";

import { revalidatePath } from "next/cache";

import { dismissAlert, undoAlertDismissal, type AlertKind } from "@/lib/api/alerts";
import { adjustItemQuantity } from "@/lib/api/items";
import { apiError } from "@/lib/api-error";

export async function snooze(itemId: string, kind: AlertKind, days: number): Promise<void> {
  await dismissAlert(itemId, kind, days);
  revalidatePath("/");
}

export async function unsnooze(dismissalId: string): Promise<void> {
  await undoAlertDismissal(dismissalId);
  revalidatePath("/");
}

export type QuantityState = { error?: string };

/** The +/- buttons on the item page and the shopping list's "bought one". */
export async function changeQuantity(itemId: string, delta: number): Promise<QuantityState> {
  const result = await adjustItemQuantity(itemId, { delta });

  if (!result.ok) {
    return {
      error: await apiError(result),
    };
  }

  revalidatePath(`/items/${itemId}`);
  revalidatePath("/");
  revalidatePath("/shopping-list");

  return {};
}
