"use server";

import { revalidatePath } from "next/cache";

import {
  createFamilyInvite,
  removeFamilyMember,
  revokeFamilyInvite,
  updateFamily,
} from "@/lib/api/family";

const SETTINGS_PATH = "/settings/family";

/** What a form action reports back to the page it was submitted from. */
export type ActionState = { error?: string; inviteUrl?: string };

export async function renameFamily(_: ActionState, form: FormData): Promise<ActionState> {
  const name = String(form.get("name") ?? "").trim();

  if (name === "") {
    return { error: "A family needs a name." };
  }

  const result = await updateFamily({ name });
  if (!result.ok) {
    return { error: describe(result.reason, result.detail) };
  }

  revalidatePath(SETTINGS_PATH);

  return {};
}

/**
 * Issues an invite and hands the link straight back to the page.
 *
 * The link is returned rather than stored because core-api keeps only its
 * digest: this response is the one and only chance to show it. It is
 * deliberately not put in the URL — a query parameter ends up in history, in
 * logs, and in whatever the browser syncs.
 */
export async function createInvite(): Promise<ActionState> {
  const result = await createFamilyInvite();
  if (!result.ok) {
    return { error: describe(result.reason, result.detail) };
  }

  revalidatePath(SETTINGS_PATH);

  return { inviteUrl: result.data.url };
}

export async function revokeInvite(form: FormData): Promise<void> {
  const id = String(form.get("inviteId") ?? "");
  if (id !== "") {
    await revokeFamilyInvite(id);
    revalidatePath(SETTINGS_PATH);
  }
}

export async function removeMember(form: FormData): Promise<void> {
  const id = String(form.get("userId") ?? "");
  if (id !== "") {
    await removeFamilyMember(id);
    revalidatePath(SETTINGS_PATH);
  }
}

/**
 * Turns a failure into something worth reading.
 *
 * core-api's own message is used when there is one, because it is the side
 * that knows what was wrong; the reason only supplies a sentence for the
 * cases where the message would be too terse to act on.
 */
function describe(reason: string, detail: string): string {
  switch (reason) {
    case "forbidden":
      return "Only the family owner can do this.";
    case "unavailable":
      return "mykomora could not reach its API. Try again in a moment.";
    default:
      return detail;
  }
}
