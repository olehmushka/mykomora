"use client";

import { useActionState } from "react";
import { useTranslations } from "next-intl";

import { changeStatus, type ActionState } from "@/app/items/[itemId]/actions";
import { Select } from "@/components/ui/select";
import type { ItemStatus } from "@/lib/api/items";

const ALL_STATUSES: ItemStatus[] = [
  "in_use",
  "stored",
  "to_give_away",
  "consumed",
  "given_away",
  "sold",
  "discarded",
  "lost",
];

const ACTIVE: ItemStatus[] = ["in_use", "stored", "to_give_away"];

/**
 * `lent` is owned by loans (lend / mark returned), so it is never offered
 * here; while an item is lent out only the archiving statuses remain.
 */
export function StatusForm({ itemId, current }: { itemId: string; current: ItemStatus }) {
  const statuses =
    current === "lent" ? ALL_STATUSES.filter((s) => !ACTIVE.includes(s)) : ALL_STATUSES;

  const t = useTranslations("ItemDetail");
  const boundChangeStatus = changeStatus.bind(null, itemId);
  const [state, formAction, pending] = useActionState<ActionState, FormData>(boundChangeStatus, {});

  return (
    <form action={formAction} className="mt-3 flex flex-wrap items-end gap-3">
      <Select
        name="status"
        defaultValue={current === "lent" ? "" : current}
        className="mt-0 w-auto"
      >
        {current === "lent" && (
          <option value="" disabled>
            {t("status.lent")}
          </option>
        )}
        {statuses.map((status) => (
          <option key={status} value={status}>
            {t(`status.${status}`)}
          </option>
        ))}
      </Select>

      <button
        type="submit"
        disabled={pending}
        className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
      >
        {pending ? t("savingButton") : t("changeStatusButton")}
      </button>

      {state.error && (
        <p role="alert" className="w-full text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}
