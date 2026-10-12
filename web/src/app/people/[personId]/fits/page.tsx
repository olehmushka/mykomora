import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { FitsList } from "@/app/people/[personId]/fits/fits-list";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getPerson } from "@/lib/api/people";
import { requireMe } from "@/lib/api/session";
import { getPersonFits } from "@/lib/api/sizes";

export const metadata = { title: "What fits — mykomora" };

/**
 * The screen M8 is about: before buying anything for a child, what already
 * fits, what fits next, and where it physically is.
 */
export default async function FitsPage({
  params,
  searchParams,
}: {
  params: Promise<{ personId: string }>;
  searchParams: Promise<{ offset?: string }>;
}) {
  const { personId } = await params;
  const { offset: rawOffset } = await searchParams;
  const offset = rawOffset === "1" ? 1 : 0;

  const me = await requireMe(`/people/${personId}/fits`);
  const t = await getTranslations("Sizes");

  const [person, fits] = await Promise.all([getPerson(personId), getPersonFits(personId, offset)]);
  if (!person.ok && person.reason === "missing") {
    notFound();
  }

  const tab = (active: boolean) =>
    `inline-flex min-h-11 items-center rounded-md px-4 text-sm ${
      active ? "bg-border/60 font-medium" : "hover:bg-border/60"
    }`;

  return (
    <AppShell me={me}>
      <Link href={`/people/${personId}`} className="text-sm text-muted hover:underline">
        {person.ok ? person.data.name : t("backLink")}
      </Link>
      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{t("fitsHeading")}</h1>

      <nav className="mt-4 flex gap-2" aria-label={t("fitsHeading")}>
        <Link
          href={`/people/${personId}/fits`}
          aria-current={offset === 0 ? "page" : undefined}
          className={tab(offset === 0)}
        >
          {t("fitsNow")}
        </Link>
        <Link
          href={`/people/${personId}/fits?offset=1`}
          aria-current={offset === 1 ? "page" : undefined}
          className={tab(offset === 1)}
        >
          {t("fitsNext")}
        </Link>
      </nav>

      {!fits.ok && (
        <div className="mt-6">
          <Notice tone="warning" title={fits.detail} />
        </div>
      )}

      {fits.ok && fits.data.sets.length === 0 && (
        <div className="mt-6">
          <Notice>{t("noFitSets")}</Notice>
        </div>
      )}

      {fits.ok &&
        fits.data.sets.map((set) => (
          <section key={set.sizeType} className="mt-8" aria-labelledby={`set-${set.sizeType}`}>
            <h2
              id={`set-${set.sizeType}`}
              className="text-sm font-medium uppercase tracking-wide text-muted"
            >
              {t(`type.${set.sizeType}`)} · {set.sizeValue} ({set.rangeMin}–{set.rangeMax})
            </h2>
            <FitsList personId={personId} items={set.items} allowOutgrown={offset === 0} />
          </section>
        ))}
    </AppShell>
  );
}
