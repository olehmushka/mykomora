"use client";

import { useActionState, useRef } from "react";
import { useTranslations } from "next-intl";

import { addCategory, type ActionState } from "@/app/settings/taxonomy/actions";

export function AddCategoryForm() {
  const t = useTranslations("Taxonomy");
  const form = useRef<HTMLFormElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await addCategory(previous, data);
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
          <label htmlFor="category-slug" className="text-sm font-medium">
            {t("slugLabel")}
          </label>
          <input
            id="category-slug"
            name="slug"
            required
            maxLength={64}
            pattern="[a-z0-9][a-z0-9_-]*"
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>

        <div className="flex-1">
          <label htmlFor="category-name" className="text-sm font-medium">
            {t("nameLabel")}
          </label>
          <input
            id="category-name"
            name="nameCustom"
            required
            maxLength={120}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>
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
