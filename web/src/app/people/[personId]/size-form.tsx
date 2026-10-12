"use client";

import { useActionState, useRef } from "react";
import { useTranslations } from "next-intl";

import { recordSize, type ActionState } from "@/app/people/[personId]/actions";
import { Select } from "@/components/ui/select";

const SYSTEMS: Record<string, string[]> = {
  clothing: ["kids_cm", "kids_eu", "intl_letter", "eu_adult"],
  shoe: ["shoe_eu", "shoe_uk", "shoe_us"],
  head: ["intl_letter", "one_size"],
};

/** Record a new size; the newest per type is the current one. */
export function SizeForm({ personId }: { personId: string }) {
  const t = useTranslations("Sizes");
  const form = useRef<HTMLFormElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await recordSize(personId, previous, data);
      if (!result.error) {
        form.current?.reset();
      }

      return result;
    },
    {},
  );

  return (
    <form
      ref={form}
      action={formAction}
      className="mt-3 flex flex-col gap-3 rounded-lg border border-border px-4 py-4 sm:flex-row sm:items-end"
    >
      <div>
        <label htmlFor="size-type" className="text-sm font-medium">
          {t("typeLabel")}
        </label>
        <Select id="size-type" name="sizeType" defaultValue="clothing">
          {Object.keys(SYSTEMS).map((type) => (
            <option key={type} value={type}>
              {t(`type.${type}`)}
            </option>
          ))}
        </Select>
      </div>

      <div>
        <label htmlFor="size-system" className="text-sm font-medium">
          {t("systemLabel")}
        </label>
        <Select id="size-system" name="sizeSystem" defaultValue="kids_cm">
          {[...new Set(Object.values(SYSTEMS).flat())].map((system) => (
            <option key={system} value={system}>
              {t(`system.${system}`)}
            </option>
          ))}
        </Select>
      </div>

      <div className="flex-1">
        <label htmlFor="size-value" className="text-sm font-medium">
          {t("valueLabel")}
        </label>
        <input
          id="size-value"
          name="sizeValue"
          required
          maxLength={32}
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
        />
      </div>

      <button
        type="submit"
        disabled={pending}
        className="inline-flex min-h-11 items-center justify-center rounded-md bg-foreground px-4 text-sm font-medium text-background disabled:opacity-60"
      >
        {t("recordButton")}
      </button>

      {state.error && (
        <p role="alert" className="text-sm text-degraded sm:w-full">
          {state.error}
        </p>
      )}
    </form>
  );
}
