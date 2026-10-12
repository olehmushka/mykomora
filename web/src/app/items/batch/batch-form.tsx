"use client";

import { useActionState, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { createBatch, type ActionState } from "@/app/items/batch/actions";
import type { FlatCategory, LocationOption } from "@/app/items/new/item-form";
import { FieldRow } from "@/components/ui/field-row";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

const SEASONS = ["summer", "winter", "demi", "all_season"] as const;

export function BatchForm({
  categories,
  locations,
  parent,
}: {
  categories: FlatCategory[];
  locations: LocationOption[];
  parent?: { id: string; name: string };
}) {
  const t = useTranslations("Batch");

  const [locationId, setLocationId] = useState(locations[0]?.id ?? "");
  const [rowKeys, setRowKeys] = useState([0, 1, 2]);
  const nextKey = useRef(3);
  const focusLast = useRef(false);
  const rowsRef = useRef<HTMLDivElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(createBatch, {});

  const location = locations.find((l) => l.id === locationId);

  // After Enter appends a row, put the cursor in its name field.
  useEffect(() => {
    if (!focusLast.current) return;

    focusLast.current = false;
    const names = rowsRef.current?.querySelectorAll<HTMLInputElement>('input[name="rowName"]');
    names?.[names.length - 1]?.focus();
  }, [rowKeys]);

  function addRow() {
    focusLast.current = true;
    setRowKeys((keys) => [...keys, nextKey.current++]);
  }

  function onRowKeyDown(event: React.KeyboardEvent<HTMLInputElement>, key: number) {
    if (event.key !== "Enter") return;

    event.preventDefault();
    if (key === rowKeys[rowKeys.length - 1]) addRow();
  }

  const errorText =
    state.error === "empty" || state.error === "unavailable" ? t(state.error) : state.error;

  return (
    <form action={formAction} className="mt-3 flex flex-col gap-6">
      <fieldset className="flex flex-col gap-3">
        <legend className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("sharedHeading")}
        </legend>

        {parent ? (
          <>
            <input type="hidden" name="parentItemId" value={parent.id} />
            <p className="text-sm text-muted">{t("insideContainer", { name: parent.name })}</p>
          </>
        ) : (
          <div className="flex flex-col gap-3 sm:flex-row">
            <FieldRow label={t("locationLabel")} htmlFor="batch-location">
              <Select
                id="batch-location"
                name="locationId"
                value={locationId}
                onChange={(event) => setLocationId(event.target.value)}
                required
              >
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </Select>
            </FieldRow>

            {location && location.rooms.length > 0 && (
              <FieldRow label={t("roomLabel")} htmlFor="batch-room">
                <Select id="batch-room" name="roomId" defaultValue="">
                  <option value="">{t("noRoom")}</option>
                  {location.rooms.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
                </Select>
              </FieldRow>
            )}
          </div>
        )}

        <div className="flex flex-col gap-3 sm:flex-row">
          <FieldRow label={t("categoryLabel")} htmlFor="batch-category">
            <Select id="batch-category" name="categoryId" required>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {"  ".repeat(c.depth)}
                  {c.name}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("seasonLabel")} htmlFor="batch-season">
            <Select id="batch-season" name="season" defaultValue="">
              <option value="">{t("noSeason")}</option>
              {SEASONS.map((s) => (
                <option key={s} value={s}>
                  {t(`season.${s}`)}
                </option>
              ))}
            </Select>
          </FieldRow>
        </div>
      </fieldset>

      <section aria-labelledby="batch-rows">
        <h2 id="batch-rows" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("rowsHeading")}
        </h2>

        <div ref={rowsRef} className="mt-2 flex flex-col gap-2">
          {rowKeys.map((key) => (
            <div
              key={key}
              className="grid grid-cols-[1fr_5rem] gap-2 sm:grid-cols-[2fr_1fr_5rem_1fr_auto]"
            >
              <Input
                name="rowName"
                aria-label={t("nameLabel")}
                placeholder={t("nameLabel")}
                maxLength={120}
                className="col-span-2 sm:col-span-1"
                onKeyDown={(event) => onRowKeyDown(event, key)}
              />
              <Input
                name="rowSize"
                aria-label={t("sizeLabel")}
                placeholder={t("sizeLabel")}
                onKeyDown={(event) => onRowKeyDown(event, key)}
              />
              <Input
                name="rowQuantity"
                type="number"
                min={0}
                step="any"
                defaultValue={1}
                aria-label={t("quantityLabel")}
                onKeyDown={(event) => onRowKeyDown(event, key)}
              />
              <Input
                name="rowBrand"
                aria-label={t("brandLabel")}
                placeholder={t("brandLabel")}
                onKeyDown={(event) => onRowKeyDown(event, key)}
              />
              <button
                type="button"
                onClick={() => setRowKeys((keys) => keys.filter((k) => k !== key))}
                disabled={rowKeys.length === 1}
                aria-label={t("removeRow")}
                className="mt-1 min-h-11 rounded-md border border-border px-3 text-sm hover:bg-border/60 disabled:opacity-40"
              >
                ×
              </button>
            </div>
          ))}
        </div>

        <button
          type="button"
          onClick={addRow}
          className="mt-3 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
        >
          {t("addRow")}
        </button>
      </section>

      {errorText && (
        <p role="alert" className="text-sm text-red-600">
          {errorText}
        </p>
      )}

      <button
        type="submit"
        disabled={pending}
        className="inline-flex min-h-11 items-center justify-center rounded-md bg-foreground px-4 text-sm font-medium text-background disabled:opacity-60 sm:self-start"
      >
        {t("submit", { count: rowKeys.length })}
      </button>
    </form>
  );
}
