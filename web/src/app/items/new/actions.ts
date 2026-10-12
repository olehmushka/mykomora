"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import {
  createItem,
  findSimilarItems,
  type SimilarItem,
  type CreateItemRequest,
  type ItemCondition,
  type ItemSeason,
  type SizeSystem,
} from "@/lib/api/items";

export type ActionState = { error?: string };

function str(form: FormData, key: string): string {
  return String(form.get(key) ?? "").trim();
}

function optStr(form: FormData, key: string): string | undefined {
  const value = str(form, key);

  return value === "" ? undefined : value;
}

function optNumber(form: FormData, key: string): number | undefined {
  const raw = optStr(form, key);
  if (raw === undefined) return undefined;

  const value = Number(raw);

  return Number.isFinite(value) ? value : undefined;
}

/**
 * Long-tail trait fields are named `trait__<key>__<dataType>` so this
 * action can coerce each one to the JSON type core-api's validator expects
 * without needing the category's trait_defs passed back in separately — the
 * dataType travels with the field itself. A bool field is true only when
 * present (an unchecked checkbox never reaches FormData at all).
 */
function collectTraits(form: FormData): Record<string, unknown> {
  const traits: Record<string, unknown> = {};

  for (const [key, raw] of form.entries()) {
    if (!key.startsWith("trait__")) continue;

    const [, traitKey, dataType] = key.split("__");
    if (!traitKey || !dataType) continue;

    if (dataType === "bool") {
      traits[traitKey] = true;
      continue;
    }

    const value = String(raw).trim();
    if (value === "") continue;

    if (dataType === "number" || dataType === "integer") {
      const n = Number(value);
      if (Number.isFinite(n)) traits[traitKey] = n;
    } else {
      traits[traitKey] = value;
    }
  }

  return traits;
}

export async function create(_: ActionState, form: FormData): Promise<ActionState> {
  const name = str(form, "name");
  if (name === "") {
    return { error: "An item needs a name." };
  }

  const categoryId = optStr(form, "categoryId");
  if (!categoryId) {
    return { error: "Choose a category." };
  }

  const quantity = optNumber(form, "quantity") ?? 1;
  if (quantity < 0) {
    return { error: "Quantity must not be negative." };
  }

  const parentItemId = optStr(form, "parentItemId");
  const locationId = optStr(form, "locationId");

  if (!parentItemId && !locationId) {
    return { error: "Choose a location." };
  }

  const body: CreateItemRequest = {
    name,
    categoryId,
    isContainer: form.get("isContainer") === "on",
    quantity,
    ...(parentItemId
      ? { parentItemId }
      : {
          locationId,
          ...(optStr(form, "roomId") ? { roomId: optStr(form, "roomId") } : {}),
          ...(optStr(form, "spot") ? { spot: optStr(form, "spot") } : {}),
        }),
    ...(optStr(form, "unit") ? { unit: optStr(form, "unit") } : {}),
    ...(optNumber(form, "minQuantity") !== undefined
      ? { minQuantity: optNumber(form, "minQuantity") }
      : {}),
    ...(optStr(form, "expiresAt") ? { expiresAt: optStr(form, "expiresAt") } : {}),
    ...(optStr(form, "openedAt") ? { openedAt: optStr(form, "openedAt") } : {}),
    ...(optNumber(form, "paoMonths") !== undefined
      ? { paoMonths: optNumber(form, "paoMonths") }
      : {}),
    ...(optStr(form, "brand") ? { brand: optStr(form, "brand") } : {}),
    ...(optStr(form, "condition") ? { condition: optStr(form, "condition") as ItemCondition } : {}),
    ...(optStr(form, "season") ? { season: optStr(form, "season") as ItemSeason } : {}),
    ...(optStr(form, "sizeSystem") ? { sizeSystem: optStr(form, "sizeSystem") as SizeSystem } : {}),
    ...(optStr(form, "sizeValue") ? { sizeValue: optStr(form, "sizeValue") } : {}),
    ...(optStr(form, "forPersonId") ? { forPersonId: optStr(form, "forPersonId") } : {}),
    ...(optStr(form, "serialNumber") ? { serialNumber: optStr(form, "serialNumber") } : {}),
    ...(optStr(form, "warrantyUntil") ? { warrantyUntil: optStr(form, "warrantyUntil") } : {}),
    ...(optStr(form, "purchaseDate") ? { purchaseDate: optStr(form, "purchaseDate") } : {}),
    ...(optNumber(form, "purchasePrice") !== undefined
      ? { purchasePrice: optNumber(form, "purchasePrice") }
      : {}),
    ...(optStr(form, "purchaseCurrency")
      ? { purchaseCurrency: optStr(form, "purchaseCurrency") }
      : {}),
    ...(optStr(form, "notes") ? { notes: optStr(form, "notes") } : {}),
  };

  const traits = collectTraits(form);
  if (Object.keys(traits).length > 0) {
    body.traits = traits;
  }

  const tagIds = form
    .getAll("tagIds")
    .map(String)
    .filter((id) => id !== "");
  if (tagIds.length > 0) {
    body.tagIds = tagIds;
  }

  const result = await createItem(body);

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath("/items");
  redirect(`/items/${result.data.id}`);
}

/**
 * Duplicate hints for the name being typed. Advisory: an error or a miss
 * simply shows nothing, because a hint must never stand between a person and
 * saving the item.
 */
export async function findSimilarAction(name: string): Promise<SimilarItem[]> {
  const trimmed = name.trim();
  if (trimmed.length < 2) return [];

  const result = await findSimilarItems(trimmed.slice(0, 120));

  return result.ok ? result.data : [];
}
