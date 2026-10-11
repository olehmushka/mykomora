"use client";

import { useActionState, useRef } from "react";
import { useTranslations } from "next-intl";

import { addLocation, type ActionState } from "@/app/locations/actions";

export function AddLocationForm() {
  const t = useTranslations("Locations");
  const form = useRef<HTMLFormElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await addLocation(previous, data);
      if (!result.error) {
        form.current?.reset();
      }

      return result;
    },
    {},
  );

  return (
    <form ref={form} action={formAction} className="mt-3 rounded-lg border border-border px-4 py-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label htmlFor="location-name" className="text-sm font-medium">
            {t("nameLabel")}
          </label>
          <input
            id="location-name"
            name="name"
            required
            maxLength={120}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>

        <div>
          <label htmlFor="location-kind" className="text-sm font-medium">
            {t("kindLabel")}
          </label>
          <select
            id="location-kind"
            name="kind"
            required
            defaultValue="home"
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          >
            <option value="home">{t("kindHome")}</option>
            <option value="storage">{t("kindStorage")}</option>
            <option value="garage">{t("kindGarage")}</option>
            <option value="relative">{t("kindRelative")}</option>
            <option value="other">{t("kindOther")}</option>
          </select>
        </div>
      </div>

      <div className="mt-3">
        <label htmlFor="location-address" className="text-sm font-medium">
          {t("addressLabel")} <span className="font-normal text-muted">{t("optionalHint")}</span>
        </label>
        <input
          id="location-address"
          name="address"
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
        />
      </div>

      <div className="mt-4 flex justify-end">
        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? t("savingButton") : t("addButton")}
        </button>
      </div>

      {state.error && (
        <p role="alert" className="mt-3 text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}
