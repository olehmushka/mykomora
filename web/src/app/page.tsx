import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { AttentionPanel } from "@/components/attention-panel";
import { listAlerts } from "@/lib/api/alerts";
import { listFamilyMembers } from "@/lib/api/family";
import { getDashboard, type Dashboard } from "@/lib/api/items";
import { requireMe } from "@/lib/api/session";
import { flattenCategories, getTaxonomy } from "@/lib/api/taxonomy";

/**
 * The inventory landing page.
 *
 * As of M5 this is the dashboard: where things are, what they cost, and what
 * changed lately. The catalogue itself is /items; every number here links
 * into it, filtered.
 */
export default async function Home() {
  const me = await requireMe("/");
  const t = await getTranslations("Dashboard");
  const locale = await getLocale();
  const [members, dashboard, taxonomy, alerts] = await Promise.all([
    listFamilyMembers(),
    getDashboard(),
    getTaxonomy(),
    listAlerts(),
  ]);

  const categoryNames = new Map(
    (taxonomy.ok ? flattenCategories(taxonomy.data.categories) : []).map((c) => [c.id, c.name]),
  );

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{me.family.name}</h1>
      <p className="mt-2 text-sm text-muted">
        {t("signedInAs", { name: me.user.displayName })}
        {me.role === "owner" && ` · ${t("ownerSuffix")}`}
      </p>

      {alerts.ok && <AttentionPanel data={alerts.data} locale={locale} />}

      <section aria-labelledby="inventory" className="mt-10">
        <h2 id="inventory" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("inventory")}
        </h2>

        {dashboard.ok ? (
          <DashboardBody
            data={dashboard.data}
            categoryNames={categoryNames}
            locale={locale}
            t={t}
          />
        ) : (
          <p className="mt-3 text-sm">{t("unavailable")}</p>
        )}

        <div className="mt-4 flex flex-wrap gap-3">
          <Link
            href="/items"
            className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            {t("browse")}
          </Link>
          <Link
            href="/items/new"
            className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            {t("add")}
          </Link>
        </div>
      </section>

      <section aria-labelledby="sharing" className="mt-10">
        <h2 id="sharing" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("sharing")}
        </h2>

        <p className="mt-3 text-sm">
          {members.ok ? t("access", { count: members.data.length }) : t("membersUnavailable")}{" "}
          <Link href="/settings/family" className="underline underline-offset-4">
            {t("familySettings")}
          </Link>
        </p>
      </section>
    </AppShell>
  );
}

type Translate = Awaited<ReturnType<typeof getTranslations<"Dashboard">>>;

const TOP_CATEGORIES = 6;

function money(total: number, currency: string, locale: string): string {
  try {
    return new Intl.NumberFormat(locale, currency ? { style: "currency", currency } : {}).format(
      total,
    );
  } catch {
    // An unrecognised currency code: show the number and the code as typed.
    return `${new Intl.NumberFormat(locale).format(total)} ${currency}`;
  }
}

function DashboardBody({
  data,
  categoryNames,
  locale,
  t,
}: {
  data: Dashboard;
  categoryNames: Map<string, string>;
  locale: string;
  t: Translate;
}) {
  if (data.totalItems === 0) {
    return <p className="mt-3 text-sm text-muted">{t("empty")}</p>;
  }

  return (
    <div className="mt-3 space-y-6">
      <p className="text-sm">{t("total", { count: data.totalItems })}</p>

      <div className="grid gap-6 sm:grid-cols-2">
        <div>
          <h3 className="text-sm font-medium">{t("byLocation")}</h3>
          <ul className="mt-2 divide-y divide-border rounded-lg border border-border">
            {data.byLocation.map((row) => (
              <li key={row.locationId}>
                <Link
                  href={`/items?locationId=${row.locationId}`}
                  className="flex min-h-11 items-center justify-between gap-3 px-4 text-sm hover:bg-border/40"
                >
                  <span className="truncate">{row.name}</span>
                  <span className="text-muted">{row.count}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h3 className="text-sm font-medium">{t("byCategory")}</h3>
          <ul className="mt-2 divide-y divide-border rounded-lg border border-border">
            {data.byCategory.slice(0, TOP_CATEGORIES).map((row) => (
              <li key={row.categoryId}>
                <Link
                  href={`/items?categoryId=${row.categoryId}`}
                  className="flex min-h-11 items-center justify-between gap-3 px-4 text-sm hover:bg-border/40"
                >
                  <span className="truncate">{categoryNames.get(row.categoryId) ?? "—"}</span>
                  <span className="text-muted">{row.count}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </div>

      {data.value.length > 0 && (
        <div>
          <h3 className="text-sm font-medium">{t("value")}</h3>
          <ul className="mt-1 text-sm">
            {data.value.map((row) => (
              <li key={row.currency}>
                {money(row.total, row.currency, locale)}{" "}
                <span className="text-muted">{t("valueCount", { count: row.itemCount })}</span>
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-muted">{t("valueHint")}</p>
        </div>
      )}

      {data.recentEvents.length > 0 && (
        <div>
          <h3 className="text-sm font-medium">{t("recent")}</h3>
          <ul className="mt-2 divide-y divide-border rounded-lg border border-border">
            {data.recentEvents.map((event) => (
              <li key={event.id}>
                <Link
                  href={`/items/${event.itemId}`}
                  className="flex min-h-11 items-center justify-between gap-3 px-4 text-sm hover:bg-border/40"
                >
                  <span className="truncate">{event.itemName}</span>
                  <span className="shrink-0 text-muted">
                    {t.has(`event.${event.type}`) ? t(`event.${event.type}`) : event.type}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
