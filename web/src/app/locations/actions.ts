"use server";

import { revalidatePath } from "next/cache";

import { archiveLocation, createLocation, type Location } from "@/lib/api/places";

const LOCATIONS_PATH = "/locations";

export type ActionState = { error?: string };

const KINDS: readonly Location["kind"][] = ["home", "storage", "garage", "relative", "other"];

export async function addLocation(_: ActionState, form: FormData): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();
  if (name === "") {
    return { error: "A location needs a name." };
  }

  const kind = String(form.get("kind") ?? "");
  if (!(KINDS as readonly string[]).includes(kind)) {
    return { error: "Choose a kind." };
  }

  const address = String(form.get("address") ?? "").trim();

  const result = await createLocation({
    name,
    kind: kind as Location["kind"],
    sortOrder: 0,
    ...(address === "" ? {} : { address }),
  });

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(LOCATIONS_PATH);

  return {};
}

export async function archive(form: FormData): Promise<void> {
  const id = String(form.get("locationId") ?? "");
  if (id !== "") {
    await archiveLocation(id);
    revalidatePath(LOCATIONS_PATH);
  }
}
