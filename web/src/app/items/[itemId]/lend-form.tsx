"use client";

import { useActionState } from "react";
import { useTranslations } from "next-intl";

import { lend, type ActionState } from "@/app/items/[itemId]/actions";
import { FieldRow } from "@/components/ui/field-row";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

/** Lend an item to a family person or to someone outside the family. */
export function LendForm({
  itemId,
  people,
}: {
  itemId: string;
  people: { id: string; name: string }[];
}) {
  const t = useTranslations("Loans");
  const boundLend = lend.bind(null, itemId);
  const [state, formAction, pending] = useActionState<ActionState, FormData>(boundLend, {});

  return (
    <form action={formAction} className="mt-3 grid gap-3 sm:grid-cols-2">
      <FieldRow label={t("borrowerPerson")} htmlFor="lend-person">
        <Select id="lend-person" name="personId" defaultValue="">
          <option value="">{t("noPerson")}</option>
          {people.map((person) => (
            <option key={person.id} value={person.id}>
              {person.name}
            </option>
          ))}
        </Select>
      </FieldRow>

      <FieldRow label={t("borrowerName")} htmlFor="lend-name" hint={t("borrowerNameHint")}>
        <Input id="lend-name" name="borrowerName" maxLength={200} />
      </FieldRow>

      <FieldRow label={t("dueAt")} htmlFor="lend-due">
        <Input id="lend-due" name="dueAt" type="date" />
      </FieldRow>

      <FieldRow label={t("note")} htmlFor="lend-note">
        <Input id="lend-note" name="note" />
      </FieldRow>

      <div className="sm:col-span-2">
        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? t("lending") : t("lendButton")}
        </button>
      </div>

      {state.error && (
        <p role="alert" className="text-sm text-degraded sm:col-span-2">
          {state.error}
        </p>
      )}
    </form>
  );
}
