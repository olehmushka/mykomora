import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listOpenLoans } from "@/lib/api/loans";
import { requireMe } from "@/lib/api/session";

/** Everything currently lent out (M9): who has it and when it is due back. */
export default async function LoansPage() {
  const me = await requireMe("/loans");
  const t = await getTranslations("Loans");
  const locale = await getLocale();
  const result = await listOpenLoans();

  const date = (value: string) =>
    new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" }).format(
      new Date(value),
    );

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("pageHeading")}</h1>
      <p className="mt-2 text-sm text-muted">{t("pageIntro")}</p>

      {!result.ok ? (
        <div className="mt-6">
          <Notice tone="warning" title={t("unavailable")} />
        </div>
      ) : result.data.length === 0 ? (
        <p className="mt-6 text-sm text-muted">{t("empty")}</p>
      ) : (
        <ul className="mt-6 divide-y divide-border rounded-lg border border-border">
          {result.data.map((loan) => (
            <li key={loan.id} className="px-4 py-3">
              <Link
                href={`/items/${loan.itemId}`}
                className="text-sm font-medium underline-offset-4 hover:underline"
              >
                {loan.itemName}
              </Link>
              <p className="mt-0.5 text-sm text-muted">
                {t("lentTo", { name: loan.borrowerName ?? "—" })}
                {" · "}
                {t("since", { date: date(loan.lentAt) })}
              </p>
              <p
                className={`mt-0.5 text-sm ${loan.overdue ? "font-medium text-degraded" : "text-muted"}`}
              >
                {loan.dueAt
                  ? loan.overdue
                    ? t("overdueSince", { date: date(loan.dueAt) })
                    : t("dueOn", { date: date(loan.dueAt) })
                  : t("noDueDate")}
              </p>
            </li>
          ))}
        </ul>
      )}
    </AppShell>
  );
}
