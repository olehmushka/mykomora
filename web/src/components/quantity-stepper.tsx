"use client";

import { useState, useTransition } from "react";
import { useTranslations } from "next-intl";

import { changeQuantity } from "@/app/attention-actions";

const buttonClass =
  "inline-flex size-11 items-center justify-center rounded-md border border-border text-lg font-medium hover:bg-border/60 disabled:opacity-60";

/** Quick +/- on a consumable's quantity; each tap is one quantity_changed event. */
export function QuantityStepper({
  itemId,
  quantity,
  unit,
}: {
  itemId: string;
  quantity: number;
  unit?: string;
}) {
  const t = useTranslations("Quantity");
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string>();

  function adjust(delta: number) {
    startTransition(async () => {
      const result = await changeQuantity(itemId, delta);
      setError(result.error);
    });
  }

  return (
    <div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          aria-label={t("decrease")}
          disabled={pending || quantity <= 0}
          onClick={() => adjust(-1)}
          className={buttonClass}
        >
          −
        </button>
        <span className="min-w-12 text-center text-sm tabular-nums" aria-live="polite">
          {Math.round(quantity * 1000) / 1000}
          {unit ? ` ${unit}` : ""}
        </span>
        <button
          type="button"
          aria-label={t("increase")}
          disabled={pending}
          onClick={() => adjust(1)}
          className={buttonClass}
        >
          +
        </button>
      </div>
      {error && (
        <p role="alert" className="mt-1 text-sm text-degraded">
          {error}
        </p>
      )}
    </div>
  );
}
