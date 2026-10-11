"use server";

import { revalidatePath } from "next/cache";

import { archiveCategory, createCategory } from "@/lib/api/taxonomy";

const TAXONOMY_PATH = "/settings/taxonomy";

export type ActionState = { error?: string };

const SLUG_PATTERN = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export async function addCategory(_: ActionState, form: FormData): Promise<ActionState> {
  const slug = String(form.get("slug") ?? "")
    .trim()
    .toLowerCase();
  if (!SLUG_PATTERN.test(slug)) {
    return { error: "A key must be lowercase letters, digits, underscores or hyphens." };
  }

  const nameCustom = String(form.get("nameCustom") ?? "").trim();
  if (nameCustom === "") {
    return { error: "A category needs a name." };
  }

  const result = await createCategory({ slug, nameCustom, sortOrder: 0 });

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(TAXONOMY_PATH);

  return {};
}

export async function archive(form: FormData): Promise<void> {
  const id = String(form.get("categoryId") ?? "");
  if (id !== "") {
    await archiveCategory(id);
    revalidatePath(TAXONOMY_PATH);
  }
}
