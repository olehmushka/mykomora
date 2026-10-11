"use client";

import { useActionState, useRef } from "react";
import { useTranslations } from "next-intl";

import { create, type ActionState } from "@/app/settings/tags/actions";

export function AddTagForm() {
  const t = useTranslations("Tags");
  const form = useRef<HTMLFormElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await create(previous, data);
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
        <label htmlFor="tag-name" className="text-sm font-medium">
          {t("nameLabel")}
        </label>
        <input
          id="tag-name"
          name="name"
          required
          maxLength={60}
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
        />
      </div>

      <div>
        <label htmlFor="tag-color" className="text-sm font-medium">
          {t("colorLabel")} <span className="font-normal text-muted">{t("optionalHint")}</span>
        </label>
        <input
          id="tag-color"
          name="color"
          placeholder="#4a7c59"
          className="mt-1 min-h-11 w-32 rounded-md border border-border bg-transparent px-3 text-sm"
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
