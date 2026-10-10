"use server";

import { revalidatePath } from "next/cache";

import { archiveRoom, createRoom } from "@/lib/api/places";

export type ActionState = { error?: string };

export async function addRoom(
  locationId: string,
  _: ActionState,
  form: FormData,
): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();
  if (name === "") {
    return { error: "A room needs a name." };
  }

  const result = await createRoom(locationId, { name, sortOrder: 0 });

  if (!result.ok) {
    return {
      error:
        result.reason === "unavailable"
          ? "mykomora could not reach its API. Try again in a moment."
          : result.detail,
    };
  }

  revalidatePath(`/locations/${locationId}`);

  return {};
}

export async function archive(locationId: string, form: FormData): Promise<void> {
  const roomId = String(form.get("roomId") ?? "");
  if (roomId !== "") {
    await archiveRoom(roomId);
    revalidatePath(`/locations/${locationId}`);
  }
}
