import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listItems, type ItemStatus } from "@/lib/api/items";
import { getLocation, getRoom } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";

const STATUS_KEYS: Record<ItemStatus, string> = {
  in_use: "status.in_use",
  stored: "status.stored",
  lent: "status.lent",
  to_give_away: "status.to_give_away",
  consumed: "status.consumed",
  given_away: "status.given_away",
  sold: "status.sold",
  discarded: "status.discarded",
  lost: "status.lost",
};

export default async function ItemsPage(props: PageProps<"/items">) {
  const sp = await props.searchParams;
  const locationId = typeof sp.locationId === "string" ? sp.locationId : undefined;
  const roomId = typeof sp.roomId === "string" ? sp.roomId : undefined;

  const me = await requireMe("/items");
  const t = await getTranslations("Items");

  const items = await listItems({ locationId, roomId });

  let heading = t("heading");

  if (roomId) {
    const room = await getRoom(roomId);
    if (room.ok) {
      heading = t("headingInRoom", { room: room.data.name });
    }
  } else if (locationId) {
    const location = await getLocation(locationId);
    if (location.ok) {
      heading = t("headingInLocation", { location: location.data.name });
    }
  }

  const newItemParams = new URLSearchParams();
  if (locationId) newItemParams.set("locationId", locationId);
  if (roomId) newItemParams.set("roomId", roomId);
  const newItemQuery = newItemParams.toString();
  const newItemHref = `/items/new${newItemQuery ? `?${newItemQuery}` : ""}`;

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{heading}</h1>

      <div className="mt-4">
        <Link
          href={newItemHref}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("addButton")}
        </Link>
      </div>

      <section aria-labelledby="list" className="mt-8">
        {!items.ok && <Notice tone="warning" title={items.detail} />}

        {items.ok && items.data.length === 0 && <Notice>{t("empty")}</Notice>}

        {items.ok && items.data.length > 0 && (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {items.data.map((item) => (
              <li
                key={item.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <Link
                    href={`/items/${item.id}`}
                    className="truncate text-sm font-medium hover:underline"
                  >
                    {item.name}
                  </Link>
                  {item.isContainer && (
                    <p className="truncate text-sm text-muted">{t("containerBadge")}</p>
                  )}
                </div>

                <span className="text-sm text-muted">{t(STATUS_KEYS[item.status])}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </AppShell>
  );
}
