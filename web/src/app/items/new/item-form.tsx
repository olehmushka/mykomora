"use client";

import { useActionState, useState } from "react";
import { useTranslations } from "next-intl";

import { create, type ActionState } from "@/app/items/new/actions";
import { NameInput } from "@/app/items/new/name-input";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldRow } from "@/components/ui/field-row";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { TraitDef } from "@/lib/api/taxonomy";

export type FlatCategory = { id: string; name: string; depth: number; traits: TraitDef[] };
export type LocationOption = { id: string; name: string; rooms: { id: string; name: string }[] };
export type PersonOption = { id: string; name: string };
export type TagOption = { id: string; name: string };

// The same mapping core-api's internal/items/traits.go keeps as the single
// source of truth for which trait keys are hot columns, not long-tail
// JSONB — duplicated here because the web and API are separate codebases,
// not because the rule itself is meant to vary between them.
const HOT_KEYS = new Set([
  "size",
  "size_system",
  "season",
  "condition",
  "for_person",
  "expires_at",
  "opened_at",
  "pao_months",
  "quantity",
  "unit",
  "min_quantity",
  "serial_number",
  "warranty_until",
]);

const SEASONS = ["summer", "winter", "demi", "all_season"] as const;
const CONDITIONS = ["new", "like_new", "good", "worn", "poor"] as const;
const SIZE_SYSTEMS = [
  "kids_cm",
  "kids_eu",
  "intl_letter",
  "eu_adult",
  "shoe_eu",
  "shoe_uk",
  "shoe_us",
  "one_size",
] as const;

export function ItemForm({
  categories,
  locations,
  people,
  tags,
  defaultLocationId,
  defaultRoomId,
  parent,
}: {
  categories: FlatCategory[];
  locations: LocationOption[];
  people: PersonOption[];
  tags: TagOption[];
  defaultLocationId?: string;
  defaultRoomId?: string;
  parent?: { id: string; name: string };
}) {
  const t = useTranslations("ItemForm");

  const [categoryId, setCategoryId] = useState(categories[0]?.id ?? "");
  const [locationId, setLocationId] = useState(defaultLocationId ?? locations[0]?.id ?? "");

  const [state, formAction, pending] = useActionState<ActionState, FormData>(create, {});

  const category = categories.find((c) => c.id === categoryId);
  const categoryTraits = category?.traits ?? [];
  const hotTraits = new Set(
    categoryTraits.filter((trait) => HOT_KEYS.has(trait.key)).map((trait) => trait.key),
  );
  const longTailTraits = categoryTraits.filter((trait) => !HOT_KEYS.has(trait.key));
  const location = locations.find((l) => l.id === locationId);

  return (
    <form action={formAction} className="mt-3 flex flex-col gap-4">
      {parent ? (
        <>
          <input type="hidden" name="parentItemId" value={parent.id} />
          <p className="text-sm text-muted">{t("insideContainer", { name: parent.name })}</p>
        </>
      ) : (
        <div className="flex flex-col gap-3 sm:flex-row">
          <FieldRow label={t("locationLabel")} htmlFor="item-location">
            <Select
              id="item-location"
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
            <FieldRow label={t("roomLabel")} htmlFor="item-room">
              <Select id="item-room" name="roomId" defaultValue={defaultRoomId ?? ""}>
                <option value="">{t("noRoom")}</option>
                {location.rooms.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </Select>
            </FieldRow>
          )}

          <FieldRow label={t("spotLabel")} htmlFor="item-spot" hint={t("optionalHint")}>
            <Input id="item-spot" name="spot" maxLength={120} />
          </FieldRow>
        </div>
      )}

      <FieldRow label={t("nameLabel")} htmlFor="item-name">
        <NameInput />
      </FieldRow>

      <FieldRow label={t("categoryLabel")} htmlFor="item-category">
        <Select
          id="item-category"
          name="categoryId"
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
          required
        >
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {"— ".repeat(c.depth)}
              {c.name}
            </option>
          ))}
        </Select>
      </FieldRow>

      <Checkbox name="isContainer" label={t("isContainerLabel")} />

      <div className="flex flex-col gap-3 sm:flex-row">
        <FieldRow label={t("quantityLabel")} htmlFor="item-quantity">
          <Input
            id="item-quantity"
            name="quantity"
            type="number"
            step="any"
            min={0}
            defaultValue={1}
            required
          />
        </FieldRow>

        {hotTraits.has("unit") && (
          <FieldRow label={t("unitLabel")} htmlFor="item-unit" hint={t("optionalHint")}>
            <Input id="item-unit" name="unit" maxLength={30} />
          </FieldRow>
        )}

        {hotTraits.has("min_quantity") && (
          <FieldRow
            label={t("minQuantityLabel")}
            htmlFor="item-min-quantity"
            hint={t("optionalHint")}
          >
            <Input id="item-min-quantity" name="minQuantity" type="number" step="any" min={0} />
          </FieldRow>
        )}
      </div>

      {hotTraits.has("season") && (
        <FieldRow label={t("seasonLabel")} htmlFor="item-season" hint={t("optionalHint")}>
          <Select id="item-season" name="season" defaultValue="">
            <option value=""></option>
            {SEASONS.map((s) => (
              <option key={s} value={s}>
                {t(`season.${s}`)}
              </option>
            ))}
          </Select>
        </FieldRow>
      )}

      {hotTraits.has("condition") && (
        <FieldRow label={t("conditionLabel")} htmlFor="item-condition" hint={t("optionalHint")}>
          <Select id="item-condition" name="condition" defaultValue="">
            <option value=""></option>
            {CONDITIONS.map((c) => (
              <option key={c} value={c}>
                {t(`condition.${c}`)}
              </option>
            ))}
          </Select>
        </FieldRow>
      )}

      {(hotTraits.has("size") || hotTraits.has("size_system")) && (
        <div className="flex flex-col gap-3 sm:flex-row">
          <FieldRow
            label={t("sizeSystemLabel")}
            htmlFor="item-size-system"
            hint={t("optionalHint")}
          >
            <Select id="item-size-system" name="sizeSystem" defaultValue="">
              <option value=""></option>
              {SIZE_SYSTEMS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </Select>
          </FieldRow>
          <FieldRow label={t("sizeValueLabel")} htmlFor="item-size-value" hint={t("optionalHint")}>
            <Input id="item-size-value" name="sizeValue" maxLength={30} />
          </FieldRow>
        </div>
      )}

      {hotTraits.has("for_person") && (
        <FieldRow label={t("forPersonLabel")} htmlFor="item-for-person" hint={t("optionalHint")}>
          <Select id="item-for-person" name="forPersonId" defaultValue="">
            <option value=""></option>
            {people.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </FieldRow>
      )}

      {(hotTraits.has("expires_at") ||
        hotTraits.has("opened_at") ||
        hotTraits.has("pao_months")) && (
        <div className="flex flex-col gap-3 sm:flex-row">
          {hotTraits.has("expires_at") && (
            <FieldRow
              label={t("expiresAtLabel")}
              htmlFor="item-expires-at"
              hint={t("optionalHint")}
            >
              <Input id="item-expires-at" name="expiresAt" type="date" />
            </FieldRow>
          )}
          {hotTraits.has("opened_at") && (
            <FieldRow label={t("openedAtLabel")} htmlFor="item-opened-at" hint={t("optionalHint")}>
              <Input id="item-opened-at" name="openedAt" type="date" />
            </FieldRow>
          )}
          {hotTraits.has("pao_months") && (
            <FieldRow
              label={t("paoMonthsLabel")}
              htmlFor="item-pao-months"
              hint={t("optionalHint")}
            >
              <Input id="item-pao-months" name="paoMonths" type="number" step="1" min={0} />
            </FieldRow>
          )}
        </div>
      )}

      {hotTraits.has("serial_number") && (
        <FieldRow
          label={t("serialNumberLabel")}
          htmlFor="item-serial-number"
          hint={t("optionalHint")}
        >
          <Input id="item-serial-number" name="serialNumber" maxLength={120} />
        </FieldRow>
      )}

      {hotTraits.has("warranty_until") && (
        <FieldRow
          label={t("warrantyUntilLabel")}
          htmlFor="item-warranty-until"
          hint={t("optionalHint")}
        >
          <Input id="item-warranty-until" name="warrantyUntil" type="date" />
        </FieldRow>
      )}

      {longTailTraits.length > 0 && (
        <div className="flex flex-col gap-3 rounded-lg border border-border px-4 py-4">
          <p className="text-sm font-medium">{t("categoryFieldsHeading")}</p>
          {longTailTraits.map((trait) => (
            <TraitField key={trait.id} trait={trait} />
          ))}
        </div>
      )}

      <div className="flex flex-col gap-3 sm:flex-row">
        <FieldRow label={t("brandLabel")} htmlFor="item-brand" hint={t("optionalHint")}>
          <Input id="item-brand" name="brand" maxLength={120} />
        </FieldRow>
        <FieldRow
          label={t("purchaseDateLabel")}
          htmlFor="item-purchase-date"
          hint={t("optionalHint")}
        >
          <Input id="item-purchase-date" name="purchaseDate" type="date" />
        </FieldRow>
        <FieldRow
          label={t("purchasePriceLabel")}
          htmlFor="item-purchase-price"
          hint={t("optionalHint")}
        >
          <Input id="item-purchase-price" name="purchasePrice" type="number" step="any" min={0} />
        </FieldRow>
      </div>

      <FieldRow label={t("notesLabel")} htmlFor="item-notes" hint={t("optionalHint")}>
        <textarea
          id="item-notes"
          name="notes"
          rows={3}
          className="mt-1 w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm"
        />
      </FieldRow>

      {tags.length > 0 && (
        <FieldRow label={t("tagsLabel")} htmlFor="item-tags" hint={t("optionalHint")}>
          <Select id="item-tags" name="tagIds" multiple className="h-auto">
            {tags.map((tag) => (
              <option key={tag.id} value={tag.id}>
                {tag.name}
              </option>
            ))}
          </Select>
        </FieldRow>
      )}

      <div>
        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? t("savingButton") : t("addButton")}
        </button>
      </div>

      {state.error && (
        <p role="alert" className="text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}

function TraitField({ trait }: { trait: TraitDef }) {
  const t = useTranslations("ItemForm");
  const tOption = useTranslations("TraitOption");
  const optionLabel = (option: string) =>
    tOption.has(`${trait.key}.${option}`) ? tOption(`${trait.key}.${option}`) : option;
  const name = `trait__${trait.key}__${trait.dataType}`;
  const id = `trait-${trait.key}`;
  const hint = trait.required ? undefined : t("optionalHint");

  if (trait.dataType === "bool") {
    return <Checkbox id={id} name={name} label={trait.label} />;
  }

  if (trait.dataType === "enum") {
    return (
      <FieldRow label={trait.label} htmlFor={id} hint={hint}>
        <Select id={id} name={name} defaultValue="" required={trait.required}>
          <option value=""></option>
          {(trait.enumOptions ?? []).map((option) => (
            <option key={option} value={option}>
              {optionLabel(option)}
            </option>
          ))}
        </Select>
      </FieldRow>
    );
  }

  const inputType =
    trait.dataType === "date"
      ? "date"
      : trait.dataType === "integer" || trait.dataType === "number"
        ? "number"
        : "text";
  const step = trait.dataType === "integer" ? "1" : trait.dataType === "number" ? "any" : undefined;

  return (
    <FieldRow label={trait.label} htmlFor={id} hint={hint}>
      <Input id={id} name={name} type={inputType} step={step} required={trait.required} />
    </FieldRow>
  );
}
