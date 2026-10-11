import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AddLocationForm } from "@/app/locations/add-location-form";
import { archive } from "@/app/locations/actions";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listLocations } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";

export const metadata = { title: "Locations — mykomora" };

/**
 * The family's physical places (SPEC 4, "Places"). Location -> room -> free
 * text spot; rooms live one level down, on each location's own page.
 */
export default async function LocationsPage() {
  const me = await requireMe("/locations");
  const t = await getTranslations("Locations");
  const locations = await listLocations();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <section aria-labelledby="add" className="mt-10">
        <h2 id="add" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("addHeading")}
        </h2>
        <AddLocationForm />
      </section>

      <section aria-labelledby="list" className="mt-10">
        {!locations.ok && (
          <Notice tone="warning" title={locations.detail}>
            {locations.reason}
          </Notice>
        )}

        {locations.ok && locations.data.length === 0 && <Notice>{t("empty")}</Notice>}

        {locations.ok && locations.data.length > 0 && (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {locations.data.map((location) => (
              <li
                key={location.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <Link
                    href={`/locations/${location.id}`}
                    className="truncate text-sm font-medium hover:underline"
                  >
                    {location.name}
                  </Link>
                  {location.address && (
                    <p className="truncate text-sm text-muted">{location.address}</p>
                  )}
                </div>

                <div className="flex items-center gap-2">
                  <Link
                    href={`/locations/${location.id}`}
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    {t("roomsLink")}
                  </Link>

                  <form action={archive}>
                    <input type="hidden" name="locationId" value={location.id} />
                    <button
                      type="submit"
                      className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                    >
                      {t("archiveButton")}
                    </button>
                  </form>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </AppShell>
  );
}
