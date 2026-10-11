import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { AddRoomForm } from "@/app/locations/[locationId]/add-room-form";
import { archive } from "@/app/locations/[locationId]/actions";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getLocation, listRooms } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";

export default async function LocationDetailPage(props: PageProps<"/locations/[locationId]">) {
  const { locationId } = await props.params;

  const me = await requireMe(`/locations/${locationId}`);
  const t = await getTranslations("LocationDetail");

  const location = await getLocation(locationId);
  if (!location.ok) {
    if (location.reason === "missing") {
      notFound();
    }

    throw new Error(location.detail);
  }

  const rooms = await listRooms(locationId);
  const archiveInThisLocation = archive.bind(null, locationId);

  return (
    <AppShell me={me}>
      <Link href="/locations" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">
        {location.data.name}
      </h1>
      {location.data.address && <p className="mt-1 text-sm text-muted">{location.data.address}</p>}

      <section aria-labelledby="rooms" className="mt-10">
        <h2 id="rooms" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("roomsHeading")}
        </h2>

        {!rooms.ok && (
          <div className="mt-3">
            <Notice tone="warning" title={rooms.detail} />
          </div>
        )}

        {rooms.ok && rooms.data.length === 0 && (
          <div className="mt-3">
            <Notice>{t("noRooms")}</Notice>
          </div>
        )}

        {rooms.ok && rooms.data.length > 0 && (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {rooms.data.map((room) => (
              <li
                key={room.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <p className="truncate text-sm font-medium">{room.name}</p>

                <form action={archiveInThisLocation}>
                  <input type="hidden" name="roomId" value={room.id} />
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    {t("archiveButton")}
                  </button>
                </form>
              </li>
            ))}
          </ul>
        )}

        <h3 className="mt-6 text-sm font-medium">{t("addRoomHeading")}</h3>
        <AddRoomForm locationId={locationId} />
      </section>
    </AppShell>
  );
}
