import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { BatchForm } from "@/app/items/batch/batch-form";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getItem } from "@/lib/api/items";
import { listLocations, listRooms } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";
import { flattenCategories, getTaxonomy } from "@/lib/api/taxonomy";

export default async function BatchItemsPage(props: PageProps<"/items/batch">) {
  const sp = await props.searchParams;
  const parentItemId = typeof sp.parentItemId === "string" ? sp.parentItemId : undefined;

  const me = await requireMe("/items/batch");
  const t = await getTranslations("Batch");

  const [taxonomy, locationsResult] = await Promise.all([getTaxonomy(), listLocations()]);

  if (!taxonomy.ok) {
    throw new Error(taxonomy.detail);
  }

  const categories = flattenCategories(taxonomy.data.categories);
  const locations = locationsResult.ok ? locationsResult.data : [];
  const locationsWithRooms = await Promise.all(
    locations.map(async (location) => {
      const rooms = await listRooms(location.id);

      return { id: location.id, name: location.name, rooms: rooms.ok ? rooms.data : [] };
    }),
  );

  let parent: { id: string; name: string } | undefined;

  if (parentItemId) {
    const parentResult = await getItem(parentItemId);
    if (parentResult.ok) {
      parent = { id: parentResult.data.id, name: parentResult.data.name };
    }
  }

  const blocked =
    categories.length === 0
      ? t("noCategories")
      : !parent && locationsWithRooms.length === 0
        ? t("noLocations")
        : "";

  return (
    <AppShell me={me}>
      <Link href="/items" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      {blocked ? (
        <div className="mt-6">
          <Notice title={blocked}>
            <Link href="/locations" className="underline underline-offset-4">
              {t("addLocationLink")}
            </Link>
          </Notice>
        </div>
      ) : (
        <BatchForm categories={categories} locations={locationsWithRooms} parent={parent} />
      )}
    </AppShell>
  );
}
