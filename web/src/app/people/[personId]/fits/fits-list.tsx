"use client";

import Image from "next/image";
import Link from "next/link";
import { useState, useTransition } from "react";
import { useTranslations } from "next-intl";

import { markOutgrown, undoOutgrown } from "@/app/people/[personId]/actions";
import type { ItemHit } from "@/lib/api/items";

function placeLabel(hit: ItemHit): string {
  const { placement } = hit;

  return [placement.room?.name, placement.location.name].filter(Boolean).join(" · ");
}

/** Items group by their physical place: the container path where there is one, else the room. */
function groupKey(hit: ItemHit): string {
  const path = hit.placement.containerPath;
  const box = path.length > 0 ? path[0]?.name : undefined;

  return box ? `${box}, ${placeLabel(hit)}` : placeLabel(hit);
}

export function FitsList({
  personId,
  items,
  allowOutgrown,
}: {
  personId: string;
  items: ItemHit[];
  allowOutgrown: boolean;
}) {
  const t = useTranslations("Sizes");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [done, setDone] = useState<{ batchId: string; changed: number }>();
  const [error, setError] = useState<string>();
  const [pending, startTransition] = useTransition();

  const groups = new Map<string, ItemHit[]>();
  for (const hit of items) {
    const key = groupKey(hit);
    groups.set(key, [...(groups.get(key) ?? []), hit]);
  }

  function toggle(id: string) {
    const next = new Set(selected);
    if (!next.delete(id)) {
      next.add(id);
    }
    setSelected(next);
  }

  function outgrown() {
    startTransition(async () => {
      const result = await markOutgrown(personId, [...selected]);
      setError(result.error);
      if (result.batchId) {
        setDone({ batchId: result.batchId, changed: result.changed ?? 0 });
        setSelected(new Set());
      }
    });
  }

  function undo() {
    if (!done) {
      return;
    }
    startTransition(async () => {
      const result = await undoOutgrown(personId, done.batchId);
      setError(result.error);
      if (!result.error) {
        setDone(undefined);
      }
    });
  }

  return (
    <div>
      {items.length === 0 && <p className="mt-3 text-sm text-muted">{t("nothingFits")}</p>}

      {[...groups.entries()].map(([place, hits]) => (
        <div key={place} className="mt-3">
          <h3 className="text-sm font-medium">{place}</h3>
          <ul className="mt-1 divide-y divide-border rounded-lg border border-border">
            {hits.map((hit) => (
              <li key={hit.item.id} className="flex items-center gap-3 px-4 py-3">
                {allowOutgrown && (
                  <input
                    type="checkbox"
                    className="size-5"
                    aria-label={hit.item.name}
                    checked={selected.has(hit.item.id)}
                    onChange={() => toggle(hit.item.id)}
                  />
                )}
                {hit.thumbUrl ? (
                  <Image
                    src={hit.thumbUrl}
                    alt=""
                    unoptimized
                    width={48}
                    height={48}
                    className="size-12 shrink-0 rounded-md object-cover"
                  />
                ) : (
                  <div
                    aria-hidden="true"
                    className="flex size-12 shrink-0 items-center justify-center rounded-md bg-border/40 text-lg text-muted"
                  >
                    {hit.item.name.slice(0, 1).toUpperCase()}
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  <Link
                    href={`/items/${hit.item.id}`}
                    className="block truncate text-sm font-medium hover:underline"
                  >
                    {hit.item.name}
                  </Link>
                  <p className="truncate text-sm text-muted">
                    {[
                      hit.item.sizeValue,
                      hit.placement.containerPath.map((c) => c.name).join(" › "),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      ))}

      {allowOutgrown && items.length > 0 && (
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <button
            type="button"
            disabled={pending || selected.size === 0}
            onClick={outgrown}
            className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
          >
            {t("outgrownButton", { count: selected.size })}
          </button>

          {done && (
            <p role="status" className="text-sm">
              {t("outgrownDone", { count: done.changed })}{" "}
              <button type="button" onClick={undo} disabled={pending} className="underline">
                {t("undo")}
              </button>
            </p>
          )}
        </div>
      )}

      {error && (
        <p role="alert" className="mt-2 text-sm text-degraded">
          {error}
        </p>
      )}
    </div>
  );
}
