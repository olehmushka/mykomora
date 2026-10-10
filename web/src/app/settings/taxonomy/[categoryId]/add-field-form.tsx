"use client";

import { useActionState, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { addField, type ActionState } from "@/app/settings/taxonomy/[categoryId]/actions";

export function AddFieldForm({ categoryId }: { categoryId: string }) {
  const t = useTranslations("CategoryDetail");
  const form = useRef<HTMLFormElement>(null);
  const [dataType, setDataType] = useState("text");
  const boundAddField = addField.bind(null, categoryId);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await boundAddField(previous, data);
      if (!result.error) {
        form.current?.reset();
        setDataType("text");
      }

      return result;
    },
    {},
  );

  return (
    <form ref={form} action={formAction} className="mt-3 rounded-lg border border-border px-4 py-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label htmlFor="field-key" className="text-sm font-medium">
            {t("keyLabel")}
          </label>
          <input
            id="field-key"
            name="key"
            required
            maxLength={64}
            pattern="[a-z0-9][a-z0-9_-]*"
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>

        <div className="flex-1">
          <label htmlFor="field-label" className="text-sm font-medium">
            {t("labelLabel")}
          </label>
          <input
            id="field-label"
            name="labelCustom"
            required
            maxLength={120}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>

        <div>
          <label htmlFor="field-type" className="text-sm font-medium">
            {t("typeLabel")}
          </label>
          <select
            id="field-type"
            name="dataType"
            value={dataType}
            onChange={(event) => setDataType(event.target.value)}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          >
            <option value="text">{t("typeText")}</option>
            <option value="number">{t("typeNumber")}</option>
            <option value="integer">{t("typeInteger")}</option>
            <option value="bool">{t("typeBool")}</option>
            <option value="date">{t("typeDate")}</option>
            <option value="enum">{t("typeEnum")}</option>
            <option value="size">{t("typeSize")}</option>
          </select>
        </div>
      </div>

      {dataType === "enum" && (
        <div className="mt-3">
          <label htmlFor="field-enum-options" className="text-sm font-medium">
            {t("enumOptionsLabel")}{" "}
            <span className="font-normal text-muted">{t("enumOptionsHint")}</span>
          </label>
          <input
            id="field-enum-options"
            name="enumOptions"
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-4">
          <label className="inline-flex min-h-11 items-center gap-2 text-sm">
            <input name="required" type="checkbox" className="size-4" />
            {t("requiredLabel")}
          </label>
          <label className="inline-flex min-h-11 items-center gap-2 text-sm">
            <input name="filterable" type="checkbox" className="size-4" />
            {t("filterableLabel")}
          </label>
        </div>

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
