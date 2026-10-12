import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { QuantityStepper } from "@/components/quantity-stepper";
import { ShoppingListExport } from "@/components/shopping-list-export";
import { getShoppingList } from "@/lib/api/alerts";
import { requireMe } from "@/lib/api/session";

/** Everything below its minimum quantity (M7), exportable as plain text. */
export default async function ShoppingListPage() {
  const me = await requireMe("/shopping-list");
  const t = await getTranslations("ShoppingList");
  const result = await getShoppingList();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>
      <p className="mt-2 text-sm text-muted">{t("intro")}</p>

      {!result.ok ? (
        <div className="mt-6">
          <Notice tone="warning" title={t("unavailable")} />
        </div>
      ) : result.data.entries.length === 0 ? (
        <p className="mt-6 text-sm text-muted">{t("empty")}</p>
      ) : (
        <>
          <ul className="mt-6 divide-y divide-border rounded-lg border border-border">
            {result.data.entries.map((entry) => (
              <li
                key={entry.itemId}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div>
                  <Link
                    href={`/items/${entry.itemId}`}
                    className="text-sm font-medium underline-offset-4 hover:underline"
                  >
                    {entry.name}
                  </Link>
                  <p className="text-sm text-muted">
                    {t("need", {
                      shortfall: Math.round((entry.shortfall ?? 0) * 1000) / 1000,
                      unit: entry.unit ?? "",
                      min: Math.round(entry.minQuantity * 1000) / 1000,
                    })}
                  </p>
                </div>
                <QuantityStepper
                  itemId={entry.itemId}
                  quantity={entry.quantity}
                  unit={entry.unit}
                />
              </li>
            ))}
          </ul>

          <ShoppingListExport entries={result.data.entries} />
        </>
      )}
    </AppShell>
  );
}
