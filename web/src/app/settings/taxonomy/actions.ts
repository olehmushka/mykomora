"use server";

import { revalidatePath } from "next/cache";

import { archiveCategory, createCategory } from "@/lib/api/taxonomy";
import { apiError, errorText } from "@/lib/api-error";

const TAXONOMY_PATH = "/settings/taxonomy";

export type ActionState = { error?: string };

const SLUG_PATTERN = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export async function addCategory(_: ActionState, form: FormData): Promise<ActionState> {
  const slug = String(form.get("slug") ?? "")
    .trim()
    .toLowerCase();
  if (!SLUG_PATTERN.test(slug)) {
    return { error: await errorText("keyFormat") };
  }

  const nameCustom = String(form.get("nameCustom") ?? "").trim();
  if (nameCustom === "") {
    return { error: await errorText("categoryName") };
  }

  const result = await createCategory({ slug, nameCustom, sortOrder: 0 });

  if (!result.ok) {
    return {
      error: await apiError(result),
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
