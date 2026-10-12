import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { snooze, unsnooze } from "@/app/attention-actions";
import type { Alert, AlertKind, AlertList } from "@/lib/api/alerts";
import { SNOOZE_PRESETS } from "@/lib/shopping-text";

const KINDS: AlertKind[] = [
  "expired",
  "loan_overdue",
  "expiring",
  "below_minimum",
  "warranty_ending",
];

const buttonClass =
  "inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60";

/**
 * What needs action today (M7). Alerts are computed by core-api on every
 * read, so this shows exactly what is true now; a snooze hides one alert for
 * the whole family until its date and can be undone from the list below.
 */
export async function AttentionPanel({ data, locale }: { data: AlertList; locale: string }) {
  const t = await getTranslations("Attention");
  const date = (value: string) =>
    new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" }).format(
      new Date(value),
    );

  const snoozed = data.snoozed ?? [];

  return (
    <section aria-labelledby="attention" className="mt-10">
      <h2 id="attention" className="text-sm font-medium uppercase tracking-wide text-muted">
        {t("heading")}
      </h2>

      {data.alerts.length === 0 ? (
        <p className="mt-3 text-sm text-muted">{t("allClear")}</p>
      ) : (
        <>
          <ul className="mt-3 flex flex-wrap gap-2" aria-label={t("summary")}>
            {KINDS.filter((kind) => data.counts[kind]).map((kind) => (
              <li
                key={kind}
                className="rounded-full border border-border px-3 py-1 text-xs font-medium"
              >
                {t(`kind.${kind}`)} · {data.counts[kind]}
              </li>
            ))}
          </ul>

          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {data.alerts.map((alert) => (
              <li key={`${alert.itemId}:${alert.kind}`} className="px-4 py-3">
                <Link
                  href={`/items/${alert.itemId}`}
                  className="text-sm font-medium underline-offset-4 hover:underline"
                >
                  {alert.itemName}
                </Link>
                <p className="mt-0.5 text-sm text-muted">{describe(alert, t, date)}</p>

                <details className="mt-2">
                  <summary className="cursor-pointer text-sm text-muted">{t("snooze")}</summary>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {SNOOZE_PRESETS.map((preset) => (
                      <form
                        key={preset.key}
                        action={snooze.bind(null, alert.itemId, alert.kind, preset.days)}
                      >
                        <button type="submit" className={buttonClass}>
                          {t(`snoozeFor.${preset.key}`)}
                        </button>
                      </form>
                    ))}
                  </div>
                </details>
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="mt-3 flex flex-wrap items-center gap-3">
        {data.counts["below_minimum"] ? (
          <Link href="/shopping-list" className={buttonClass}>
            {t("shoppingList")}
          </Link>
        ) : null}
      </div>

      {snoozed.length > 0 && (
        <details className="mt-3">
          <summary className="cursor-pointer text-sm text-muted">
            {t("snoozedSummary", { count: snoozed.length })}
          </summary>
          <ul className="mt-2 divide-y divide-border rounded-lg border border-border">
            {snoozed.map((d) => (
              <li
                key={d.id}
                className="flex flex-wrap items-center justify-between gap-2 px-4 py-2"
              >
                <span className="text-sm">
                  {d.itemName ?? "—"} · {t(`kind.${d.kind}`)}
                  <span className="text-muted">
                    {" "}
                    · {t("until", { date: date(d.snoozedUntil) })}
                  </span>
                </span>
                <form action={unsnooze.bind(null, d.id)}>
                  <button type="submit" className={buttonClass}>
                    {t("undo")}
                  </button>
                </form>
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}

function describe(
  alert: Alert,
  t: Awaited<ReturnType<typeof getTranslations>>,
  date: (value: string) => string,
): string {
  switch (alert.kind) {
    case "expired":
      return t("expiredOn", { date: date(alert.dueDate ?? "") });
    case "expiring":
      return t("expiresOn", { date: date(alert.dueDate ?? "") });
    case "warranty_ending":
      return t("warrantyEnds", { date: date(alert.dueDate ?? "") });
    case "loan_overdue":
      return t("loanOverdue", { date: date(alert.dueDate ?? ""), borrower: alert.borrower ?? "—" });
    case "below_minimum":
      return t("belowMinimum", {
        quantity: Math.round((alert.quantity ?? 0) * 1000) / 1000,
        min: Math.round((alert.minQuantity ?? 0) * 1000) / 1000,
        unit: alert.unit ?? "",
      });
  }
}
