"use client";

import { useActionState, useState } from "react";
import { useTranslations } from "next-intl";

import { move, type ActionState } from "@/app/items/[itemId]/actions";
import { FieldRow } from "@/components/ui/field-row";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

export function MoveForm({
  itemId,
  locations,
  containers,
}: {
  itemId: string;
  locations: { id: string; name: string; rooms: { id: string; name: string }[] }[];
  containers: { id: string; name: string }[];
}) {
  const t = useTranslations("ItemDetail");
  const boundMove = move.bind(null, itemId);
  const [state, formAction, pending] = useActionState<ActionState, FormData>(boundMove, {});
  const [mode, setMode] = useState<"place" | "container">("place");
  const [locationId, setLocationId] = useState(locations[0]?.id ?? "");

  const location = locations.find((l) => l.id === locationId);

  return (
    <form action={formAction} className="mt-3 flex flex-col gap-3">
      {containers.length > 0 && (
        <div className="flex gap-4 text-sm">
          <label className="inline-flex min-h-11 items-center gap-2">
            <input
              type="radio"
              name="mode"
              checked={mode === "place"}
              onChange={() => setMode("place")}
              className="size-4"
            />
            {t("moveToPlace")}
          </label>
          <label className="inline-flex min-h-11 items-center gap-2">
            <input
              type="radio"
              name="mode"
              checked={mode === "container"}
              onChange={() => setMode("container")}
              className="size-4"
            />
            {t("moveIntoContainer")}
          </label>
        </div>
      )}

      {mode === "place" ? (
        <div className="flex flex-col gap-3 sm:flex-row">
          <FieldRow label={t("locationLabel")} htmlFor="move-location">
            <Select
              id="move-location"
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
            <FieldRow label={t("roomLabel")} htmlFor="move-room">
              <Select id="move-room" name="roomId" defaultValue="">
                <option value="">{t("noRoom")}</option>
                {location.rooms.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </Select>
            </FieldRow>
          )}

          <FieldRow label={t("spotLabel")} htmlFor="move-spot" hint={t("optionalHint")}>
            <Input id="move-spot" name="spot" maxLength={120} />
          </FieldRow>
        </div>
      ) : (
        <FieldRow label={t("containerLabel")} htmlFor="move-container">
          <Select id="move-container" name="parentItemId" defaultValue="">
            <option value="" disabled>
              {t("chooseContainer")}
            </option>
            {containers.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
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
          {pending ? t("savingButton") : t("moveButton")}
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
