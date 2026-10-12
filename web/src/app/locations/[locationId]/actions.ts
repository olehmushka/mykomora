"use server";

import { revalidatePath } from "next/cache";

import { archiveRoom, createRoom } from "@/lib/api/places";
import { apiError, errorText } from "@/lib/api-error";

export type ActionState = { error?: string };

export async function addRoom(
  locationId: string,
  _: ActionState,
  form: FormData,
): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();
  if (name === "") {
    return { error: await errorText("roomName") };
  }

  const result = await createRoom(locationId, { name, sortOrder: 0 });

  if (!result.ok) {
    return {
      error: await apiError(result),
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
