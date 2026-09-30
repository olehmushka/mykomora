"use server";

import { revalidatePath } from "next/cache";

import { archivePerson, createPerson } from "@/lib/api/people";

const PEOPLE_PATH = "/people";

export type ActionState = { error?: string };

export async function addPerson(_: ActionState, form: FormData): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();
  if (name === "") {
    return { error: "A person needs a name." };
  }

  const birthdate = String(form.get("birthdate") ?? "").trim();

  const result = await createPerson({
    name,
    isChild: form.get("isChild") === "on",
    // An empty date input submits an empty string; the field is optional, and
    // absent is not the same as blank.
    ...(birthdate === "" ? {} : { birthdate }),
  });

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(PEOPLE_PATH);

  return {};
}

export async function archive(form: FormData): Promise<void> {
  const id = String(form.get("personId") ?? "");
  if (id !== "") {
    await archivePerson(id);
    revalidatePath(PEOPLE_PATH);
  }
}
