import type { Metadata } from "next";
import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { archive } from "@/app/people/actions";
import { AddPersonForm } from "@/app/people/add-person-form";
import { listPeople } from "@/lib/api/people";
import { requireMe } from "@/lib/api/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("People");
  return { title: t("metaTitle") };
}

/**
 * The people a family tracks things for.
 *
 * Not the same as the members who can sign in: the second-priority job of this
 * application is knowing what fits the children, and children do not have
 * Google accounts (SPEC 3).
 */
export default async function PeoplePage() {
  const t = await getTranslations("People");
  const locale = await getLocale();
  const dates = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" });
  const me = await requireMe("/people");
  const people = await listPeople();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("title")}</h1>
      <p className="mt-2 text-sm text-muted">{t("intro")}</p>

      <section aria-labelledby="add" className="mt-10">
        <h2 id="add" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("addHeading")}
        </h2>
        <AddPersonForm />
      </section>

      <section aria-labelledby="list" className="mt-10">
        <h2 id="list" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("listHeading")}
        </h2>

        {!people.ok && (
          <div className="mt-3">
            <Notice tone="warning" title={t("unavailable")}>
              {people.detail}
            </Notice>
          </div>
        )}

        {people.ok && people.data.length === 0 && (
          <div className="mt-3">
            <Notice title={t("emptyTitle")}>{t("emptyDetail")}</Notice>
          </div>
        )}

        {people.ok && people.data.length > 0 && (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {people.data.map((person) => (
              <li
                key={person.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <Link
                    href={`/people/${person.id}`}
                    className="truncate text-sm font-medium hover:underline"
                  >
                    {person.name}
                  </Link>
                  <p className="truncate text-sm text-muted">
                    {person.isChild ? t("child") : t("adult")}
                    {person.birthdate
                      ? ` · ${t("born", { date: dates.format(new Date(person.birthdate)) })}`
                      : ""}
                  </p>
                </div>

                <form action={archive}>
                  <input type="hidden" name="personId" value={person.id} />
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    {t("archive")}
                  </button>
                </form>
              </li>
            ))}
          </ul>
        )}
      </section>
    </AppShell>
  );
}
