"use client";

import { useActionState, useRef } from "react";
import { useTranslations } from "next-intl";

import { addRoom, type ActionState } from "@/app/locations/[locationId]/actions";

export function AddRoomForm({ locationId }: { locationId: string }) {
  const t = useTranslations("LocationDetail");
  const form = useRef<HTMLFormElement>(null);
  const boundAddRoom = addRoom.bind(null, locationId);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await boundAddRoom(previous, data);
      if (!result.error) {
        form.current?.reset();
      }

      return result;
    },
    {},
  );

  return (
    <form ref={form} action={formAction} className="mt-3 flex flex-wrap items-end gap-3">
      <div className="flex-1">
        <label htmlFor="room-name" className="text-sm font-medium">
          {t("roomNameLabel")}
        </label>
        <input
          id="room-name"
          name="name"
          required
          maxLength={120}
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
        />
      </div>

      <button
        type="submit"
        disabled={pending}
        className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
      >
        {pending ? t("savingButton") : t("addButton")}
      </button>

      {state.error && (
        <p role="alert" className="w-full text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}
