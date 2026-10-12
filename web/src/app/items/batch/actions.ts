"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { createItemsBatch, type CreateItemRequest, type ItemSeason } from "@/lib/api/items";

export type ActionState = { error?: string };

function optStr(form: FormData, key: string): string | undefined {
  const value = String(form.get(key) ?? "").trim();

  return value === "" ? undefined : value;
}

/**
 * Shared fields come once; the variable columns (`rowName`, `rowSize`,
 * `rowQuantity`, `rowBrand`) come as parallel lists. Rows with no name are
 * skipped, so the trailing empty row never has to be removed by hand.
 */
export async function createBatch(_: ActionState, form: FormData): Promise<ActionState> {
  const categoryId = optStr(form, "categoryId");
  const parentItemId = optStr(form, "parentItemId");
  const locationId = optStr(form, "locationId");
  const roomId = optStr(form, "roomId");
  const season = optStr(form, "season") as ItemSeason | undefined;

  const names = form.getAll("rowName").map(String);
  const sizes = form.getAll("rowSize").map(String);
  const quantities = form.getAll("rowQuantity").map(String);
  const brands = form.getAll("rowBrand").map(String);

  const items: CreateItemRequest[] = [];

  names.forEach((rawName, i) => {
    const name = rawName.trim();
    if (name === "") return;

    const quantity = Number(quantities[i]);
    const size = (sizes[i] ?? "").trim();
    const brand = (brands[i] ?? "").trim();

    items.push({
      name,
      categoryId: categoryId ?? "",
      isContainer: false,
      quantity: Number.isFinite(quantity) && quantity >= 0 ? quantity : 1,
      ...(parentItemId ? { parentItemId } : { locationId, ...(roomId ? { roomId } : {}) }),
      ...(season ? { season } : {}),
      ...(size ? { sizeValue: size } : {}),
      ...(brand ? { brand } : {}),
    });
  });

  if (items.length === 0) {
    return { error: "empty" };
  }

  const result = await createItemsBatch(items);

  if (!result.ok) {
    return { error: result.reason === "unavailable" ? "unavailable" : result.detail };
  }

  revalidatePath("/items");
  redirect(parentItemId ? `/items/${parentItemId}` : "/items");
}
