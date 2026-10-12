import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import { SizeForm } from "@/app/people/[personId]/size-form";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getPerson } from "@/lib/api/people";
import { requireMe } from "@/lib/api/session";
import { listPersonSizes, type PersonSize } from "@/lib/api/sizes";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Meta");

  return { title: t("person") };
}

/** Current size per type: the history is newest first, so the first of each type wins. */
function currentSizes(history: PersonSize[]): PersonSize[] {
  const seen = new Set<string>();

  return history.filter((size) => {
    if (seen.has(size.sizeType)) {
      return false;
    }
    seen.add(size.sizeType);

    return true;
  });
}

export default async function PersonPage({ params }: { params: Promise<{ personId: string }> }) {
  const { personId } = await params;
  const me = await requireMe(`/people/${personId}`);
  const t = await getTranslations("Sizes");
  const locale = await getLocale();

  const [person, sizes] = await Promise.all([getPerson(personId), listPersonSizes(personId)]);
  if (!person.ok) {
    if (person.reason === "missing") {
      notFound();
    }

    return (
      <AppShell me={me}>
        <Notice tone="warning" title={person.detail} />
      </AppShell>
    );
  }

  const history = sizes.ok ? sizes.data : [];
  const current = currentSizes(history);
  const date = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });

  return (
    <AppShell me={me}>
      <Link href="/people" className="text-sm text-muted hover:underline">
        {t("backLink")}
      </Link>
      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{person.data.name}</h1>

      <div className="mt-4 flex flex-wrap gap-2">
        <Link
          href={`/people/${personId}/fits`}
          className="inline-flex min-h-11 items-center rounded-md bg-foreground px-4 text-sm font-medium text-background"
        >
          {t("whatFitsButton")}
        </Link>
        <Link
          href={`/people/${personId}/fits?offset=1`}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("whatFitsNextButton")}
        </Link>
      </div>

      <section aria-labelledby="current" className="mt-10">
        <h2 id="current" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("currentHeading")}
        </h2>
        {current.length === 0 ? (
          <div className="mt-3">
            <Notice>{t("noSizes")}</Notice>
          </div>
        ) : (
          <ul className="mt-3 grid gap-3 sm:grid-cols-3">
            {current.map((size) => (
              <li key={size.id} className="rounded-lg border border-border px-4 py-3">
                <p className="text-xs uppercase tracking-wide text-muted">
                  {t(`type.${size.sizeType}`)}
                </p>
                <p className="text-lg font-semibold">{size.sizeValue}</p>
                <p className="text-xs text-muted">{t(`system.${size.sizeSystem}`)}</p>
                {size.sizeNormMin === undefined && (
                  <p className="mt-1 text-xs text-degraded">{t("unmapped")}</p>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="record" className="mt-10">
        <h2 id="record" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("recordHeading")}
        </h2>
        <SizeForm personId={personId} />
      </section>

      {history.length > 0 && (
        <section aria-labelledby="history" className="mt-10">
          <h2 id="history" className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("historyHeading")}
          </h2>
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {history.map((size) => (
              <li key={size.id} className="flex items-center justify-between gap-3 px-4 py-3">
                <span className="text-sm">
                  {t(`type.${size.sizeType}`)} · {size.sizeValue}{" "}
                  <span className="text-muted">({t(`system.${size.sizeSystem}`)})</span>
                </span>
                <span className="text-sm text-muted">{date.format(new Date(size.recordedAt))}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </AppShell>
  );
}
