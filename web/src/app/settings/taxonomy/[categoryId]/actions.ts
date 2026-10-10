"use server";

import { revalidatePath } from "next/cache";

import { archiveTraitDef, createTraitDef, type TraitDef } from "@/lib/api/taxonomy";

const TAXONOMY_PATH = "/settings/taxonomy";

export type ActionState = { error?: string };

const KEY_PATTERN = /^[a-z0-9][a-z0-9_-]{0,63}$/;
const DATA_TYPES: readonly TraitDef["dataType"][] = [
  "text",
  "number",
  "integer",
  "bool",
  "date",
  "enum",
  "size",
];

export async function addField(
  categoryId: string,
  _: ActionState,
  form: FormData,
): Promise<ActionState> {
  const key = String(form.get("key") ?? "")
    .trim()
    .toLowerCase();
  if (!KEY_PATTERN.test(key)) {
    return { error: "A key must be lowercase letters, digits, underscores or hyphens." };
  }

  const labelCustom = String(form.get("labelCustom") ?? "").trim();
  if (labelCustom === "") {
    return { error: "A field needs a label." };
  }

  const dataType = String(form.get("dataType") ?? "");
  if (!(DATA_TYPES as readonly string[]).includes(dataType)) {
    return { error: "Choose a type." };
  }

  const rawOptions = String(form.get("enumOptions") ?? "");
  const enumOptions = rawOptions
    .split(",")
    .map((option) => option.trim())
    .filter((option) => option !== "");

  if (dataType === "enum" && enumOptions.length === 0) {
    return { error: "A choice field needs at least one comma-separated option." };
  }

  const result = await createTraitDef(categoryId, {
    key,
    labelCustom,
    dataType: dataType as TraitDef["dataType"],
    required: form.get("required") === "on",
    filterable: form.get("filterable") === "on",
    sortOrder: 0,
    ...(dataType === "enum" ? { enumOptions } : {}),
  });

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(`${TAXONOMY_PATH}/${categoryId}`);

  return {};
}

export async function archive(categoryId: string, form: FormData): Promise<void> {
  const traitDefId = String(form.get("traitDefId") ?? "");
  if (traitDefId !== "") {
    await archiveTraitDef(traitDefId);
    revalidatePath(`${TAXONOMY_PATH}/${categoryId}`);
  }
}
