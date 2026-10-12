"use server";

import { revalidatePath } from "next/cache";

import { createTag, deleteTag, updateTag } from "@/lib/api/tags";
import { apiError, errorText } from "@/lib/api-error";

const TAGS_PATH = "/settings/tags";

export type ActionState = { error?: string };

export async function create(_: ActionState, form: FormData): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();
  if (name === "") {
    return { error: await errorText("tagName") };
  }

  const color = String(form.get("color") ?? "").trim();

  const result = await createTag({ name, ...(color ? { color } : {}) });

  if (!result.ok) {
    return {
      error: await apiError(result),
    };
  }

  revalidatePath(TAGS_PATH);

  return {};
}

export async function rename(form: FormData): Promise<void> {
  const tagId = String(form.get("tagId") ?? "");
  const name = String(form.get("name") ?? "").trim();
  if (tagId === "" || name === "") {
    return;
  }

  const color = String(form.get("color") ?? "").trim();

  await updateTag(tagId, { name, ...(color ? { color } : {}) });
  revalidatePath(TAGS_PATH);
}

export async function remove(form: FormData): Promise<void> {
  const tagId = String(form.get("tagId") ?? "");
  if (tagId !== "") {
    await deleteTag(tagId);
    revalidatePath(TAGS_PATH);
  }
}
