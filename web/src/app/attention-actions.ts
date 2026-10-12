"use server";

import { revalidatePath } from "next/cache";

import { dismissAlert, undoAlertDismissal, type AlertKind } from "@/lib/api/alerts";
import { adjustItemQuantity } from "@/lib/api/items";

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
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(`/items/${itemId}`);
  revalidatePath("/");
  revalidatePath("/shopping-list");

  return {};
}
