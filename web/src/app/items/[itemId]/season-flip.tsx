"use client";

import { useState, useTransition } from "react";
import { useTranslations } from "next-intl";

import { flipSeason, undoFlip } from "@/app/items/[itemId]/actions";

const buttonClass =
  "inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60";

/** Seasonal pack/unpack for a container: one tap, one event per item, one undo. */
export function SeasonFlip({ itemId }: { itemId: string }) {
  const t = useTranslations("Sizes");
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string>();
  const [last, setLast] = useState<{ batchId: string; changed: number }>();

  function flip(direction: "pack" | "unpack") {
    startTransition(async () => {
      const result = await flipSeason(itemId, direction);
      setError(result.error);
      setLast(
        result.batchId ? { batchId: result.batchId, changed: result.changed ?? 0 } : undefined,
      );
    });
  }

  function undo() {
    if (!last) {
      return;
    }
    startTransition(async () => {
      const result = await undoFlip(itemId, last.batchId);
      setError(result.error);
      if (!result.error) {
        setLast(undefined);
      }
    });
  }

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      <button type="button" disabled={pending} onClick={() => flip("pack")} className={buttonClass}>
        {t("packButton")}
      </button>
      <button
        type="button"
        disabled={pending}
        onClick={() => flip("unpack")}
        className={buttonClass}
      >
        {t("unpackButton")}
      </button>

      {last && (
        <p role="status" className="text-sm">
          {t("flipDone", { count: last.changed })}{" "}
          <button type="button" onClick={undo} disabled={pending} className="underline">
            {t("undo")}
          </button>
        </p>
      )}

      {error && (
        <p role="alert" className="w-full text-sm text-degraded">
          {error}
        </p>
      )}
    </div>
  );
}
