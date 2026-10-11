import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ItemForm, type FlatCategory } from "@/app/items/new/item-form";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getItem } from "@/lib/api/items";
import { listPeople } from "@/lib/api/people";
import { listLocations, listRooms } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";
import { getTaxonomy, type Category } from "@/lib/api/taxonomy";
import { listTags } from "@/lib/api/tags";

function flattenCategories(categories: Category[], depth = 0): FlatCategory[] {
  const out: FlatCategory[] = [];

  for (const category of categories) {
    out.push({ id: category.id, name: category.name, depth, traits: category.traits });
    out.push(...flattenCategories(category.children, depth + 1));
  }

  return out;
}

export default async function NewItemPage(props: PageProps<"/items/new">) {
  const sp = await props.searchParams;
  const parentItemId = typeof sp.parentItemId === "string" ? sp.parentItemId : undefined;
  const defaultLocationId = typeof sp.locationId === "string" ? sp.locationId : undefined;
  const defaultRoomId = typeof sp.roomId === "string" ? sp.roomId : undefined;

  const me = await requireMe("/items/new");
  const t = await getTranslations("ItemForm");

  const [taxonomy, locationsResult, peopleResult, tagsResult] = await Promise.all([
    getTaxonomy(),
    listLocations(),
    listPeople(),
    listTags(),
  ]);

  if (!taxonomy.ok) {
    throw new Error(taxonomy.detail);
  }

  const categories = flattenCategories(taxonomy.data.categories);

  if (categories.length === 0) {
    return (
      <AppShell me={me}>
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>
        <div className="mt-6">
          <Notice title={t("noCategories")} />
        </div>
      </AppShell>
    );
  }

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

  if (!parent && locationsWithRooms.length === 0) {
    return (
      <AppShell me={me}>
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>
        <div className="mt-6">
          <Notice title={t("noLocations")}>
            <Link href="/locations" className="underline underline-offset-4">
              {t("addLocationLink")}
            </Link>
          </Notice>
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell me={me}>
      <Link href="/items" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <ItemForm
        categories={categories}
        locations={locationsWithRooms}
        people={peopleResult.ok ? peopleResult.data : []}
        tags={tagsResult.ok ? tagsResult.data : []}
        defaultLocationId={defaultLocationId}
        defaultRoomId={defaultRoomId}
        parent={parent}
      />
    </AppShell>
  );
}
