import { getTranslations } from "next-intl/server";

import { SearchBar } from "@/app/items/search-bar";
import { TRAIT_PREFIX, type FilterValues } from "@/app/items/search-params";
import { FieldRow } from "@/components/ui/field-row";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { ItemStatus } from "@/lib/api/items";
import type { FlatCategory } from "@/lib/api/taxonomy";

const STATUSES: ItemStatus[] = [
  "in_use",
  "stored",
  "lent",
  "to_give_away",
  "consumed",
  "given_away",
  "sold",
  "discarded",
  "lost",
];
const SEASONS = ["summer", "winter", "demi", "all_season"] as const;
const CONDITIONS = ["new", "like_new", "good", "worn", "poor"] as const;

export type FilterLookups = {
  categories: FlatCategory[];
  locations: { id: string; name: string; rooms: { id: string; name: string }[] }[];
  people: { id: string; name: string }[];
  tags: { id: string; name: string }[];
};

/**
 * The search box and filters, as one plain GET form: submitting it rewrites
 * the URL, and the page re-renders from that. It works without JavaScript;
 * the debounced search box just saves the press of Enter.
 *
 * Trait filters come from the selected category's `filterable` fields, so a
 * family's own filterable custom fields appear here with no code change.
 * Choosing a different category and pressing "Apply" is what reveals them.
 */
export async function FilterPanel({
  values,
  lookups,
  open,
}: {
  values: FilterValues;
  lookups: FilterLookups;
  open: boolean;
}) {
  const t = await getTranslations("Search");
  const tItem = await getTranslations("ItemForm");
  const tItems = await getTranslations("Items");

  const category = lookups.categories.find((c) => c.id === values.categoryId);
  const traitDefs = (category?.traits ?? []).filter(
    (def) => def.filterable && ["enum", "bool", "text", "number", "integer"].includes(def.dataType),
  );

  return (
    <form action="/items" method="get" role="search" className="mt-6">
      {values.view && <input type="hidden" name="view" value={values.view} />}

      <div className="flex gap-2">
        <div className="flex-1">
          <SearchBar initial={values.q ?? ""} />
        </div>
        <button
          type="submit"
          className="mt-1 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("apply")}
        </button>
      </div>

      <details className="mt-3 rounded-lg border border-border px-4 py-2" open={open}>
        <summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">
          {t("filters")}
        </summary>

        <div className="grid gap-4 pb-3 pt-1 sm:grid-cols-2">
          <FieldRow label={t("category")} htmlFor="f-category">
            <Select id="f-category" name="categoryId" defaultValue={values.categoryId ?? ""}>
              <option value="">{t("any")}</option>
              {lookups.categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {`${"  ".repeat(c.depth)}${c.name}`}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("status")} htmlFor="f-status">
            <Select id="f-status" name="status" defaultValue={values.status ?? ""}>
              <option value="">{t("any")}</option>
              {STATUSES.map((status) => (
                <option key={status} value={status}>
                  {tItems(`status.${status}`)}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("room")} htmlFor="f-room">
            <Select id="f-room" name="roomId" defaultValue={values.roomId ?? ""}>
              <option value="">{t("any")}</option>
              {lookups.locations.map((location) => (
                <optgroup key={location.id} label={location.name}>
                  {location.rooms.map((room) => (
                    <option key={room.id} value={room.id}>
                      {room.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("location")} htmlFor="f-location">
            <Select id="f-location" name="locationId" defaultValue={values.locationId ?? ""}>
              <option value="">{t("any")}</option>
              {lookups.locations.map((location) => (
                <option key={location.id} value={location.id}>
                  {location.name}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("person")} htmlFor="f-person">
            <Select id="f-person" name="personId" defaultValue={values.personId ?? ""}>
              <option value="">{t("any")}</option>
              {lookups.people.map((person) => (
                <option key={person.id} value={person.id}>
                  {person.name}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={t("tag")} htmlFor="f-tag">
            <Select id="f-tag" name="tagId" defaultValue={values.tagId ?? ""}>
              <option value="">{t("any")}</option>
              {lookups.tags.map((tag) => (
                <option key={tag.id} value={tag.id}>
                  {tag.name}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={tItem("seasonLabel")} htmlFor="f-season">
            <Select id="f-season" name="season" defaultValue={values.season ?? ""}>
              <option value="">{t("any")}</option>
              {SEASONS.map((season) => (
                <option key={season} value={season}>
                  {tItem(`season.${season}`)}
                </option>
              ))}
            </Select>
          </FieldRow>

          <FieldRow label={tItem("conditionLabel")} htmlFor="f-condition">
            <Select id="f-condition" name="condition" defaultValue={values.condition ?? ""}>
              <option value="">{t("any")}</option>
              {CONDITIONS.map((condition) => (
                <option key={condition} value={condition}>
                  {tItem(`condition.${condition}`)}
                </option>
              ))}
            </Select>
          </FieldRow>

          <RangeRow
            label={t("expires")}
            fromName="expiresFrom"
            toName="expiresTo"
            type="date"
            values={values}
            fromLabel={t("from")}
            toLabel={t("to")}
          />
          <RangeRow
            label={t("warranty")}
            fromName="warrantyFrom"
            toName="warrantyTo"
            type="date"
            values={values}
            fromLabel={t("from")}
            toLabel={t("to")}
          />
          <RangeRow
            label={t("price")}
            fromName="priceMin"
            toName="priceMax"
            type="number"
            values={values}
            fromLabel={t("from")}
            toLabel={t("to")}
          />

          <FieldRow label={t("sort")} htmlFor="f-sort">
            <Select id="f-sort" name="sort" defaultValue={values.sort ?? ""}>
              <option value="">{t("sortDefault")}</option>
              <option value="name">{t("sortName")}</option>
              <option value="expires">{t("sortExpires")}</option>
              <option value="price">{t("sortPrice")}</option>
            </Select>
          </FieldRow>
        </div>

        {traitDefs.length > 0 && (
          <div className="grid gap-4 border-t border-border pb-3 pt-3 sm:grid-cols-2">
            {traitDefs.map((def) => {
              const name = `${TRAIT_PREFIX}${def.key}`;
              const id = `f-${name}`;
              const current = values[name] ?? "";

              if (def.dataType === "enum" || def.dataType === "bool") {
                const options =
                  def.dataType === "bool"
                    ? [{ value: "true", label: t("yes") }]
                    : (def.enumOptions ?? []).map((o) => ({ value: o, label: o }));

                return (
                  <FieldRow key={def.id} label={def.label} htmlFor={id}>
                    <Select id={id} name={name} defaultValue={current}>
                      <option value="">{t("any")}</option>
                      {options.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </Select>
                  </FieldRow>
                );
              }

              return (
                <FieldRow key={def.id} label={def.label} htmlFor={id}>
                  <Input
                    id={id}
                    name={name}
                    defaultValue={current}
                    type={def.dataType === "text" ? "text" : "number"}
                    step="any"
                  />
                </FieldRow>
              );
            })}
          </div>
        )}
      </details>
    </form>
  );
}

function RangeRow({
  label,
  fromName,
  toName,
  type,
  values,
  fromLabel,
  toLabel,
}: {
  label: string;
  fromName: string;
  toName: string;
  type: "date" | "number";
  values: FilterValues;
  fromLabel: string;
  toLabel: string;
}) {
  return (
    <fieldset className="flex-1">
      <legend className="text-sm font-medium">{label}</legend>
      <div className="flex gap-2">
        <Input
          type={type}
          name={fromName}
          defaultValue={values[fromName] ?? ""}
          aria-label={`${label}: ${fromLabel}`}
          step="any"
        />
        <Input
          type={type}
          name={toName}
          defaultValue={values[toName] ?? ""}
          aria-label={`${label}: ${toLabel}`}
          step="any"
        />
      </div>
    </fieldset>
  );
}
