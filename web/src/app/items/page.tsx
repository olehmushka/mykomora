import Image from "next/image";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { FilterPanel } from "@/app/items/filter-panel";
import {
  hasActiveFilters,
  hrefWith,
  readFilterValues,
  toApiParams,
} from "@/app/items/search-params";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listItems, type ItemHit, type ItemStatus } from "@/lib/api/items";
import { listPeople } from "@/lib/api/people";
import { listLocations, listRooms } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";
import { listTags } from "@/lib/api/tags";
import { flattenCategories, getTaxonomy } from "@/lib/api/taxonomy";

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

const THUMB_PX = 112;
const PRICE_AND_DATE_FILTERS = [
  "priceMin",
  "priceMax",
  "expiresFrom",
  "expiresTo",
  "warrantyFrom",
  "warrantyTo",
];

export default async function ItemsPage(props: PageProps<"/items">) {
  const values = readFilterValues(await props.searchParams);

  const me = await requireMe("/items");
  const t = await getTranslations("Items");
  const ts = await getTranslations("Search");

  const [taxonomy, locationsResult, peopleResult, tagsResult] = await Promise.all([
    getTaxonomy(),
    listLocations(),
    listPeople(),
    listTags(),
  ]);

  const categories = taxonomy.ok ? flattenCategories(taxonomy.data.categories) : [];
  const selectedCategory = categories.find((c) => c.id === values.categoryId);

  const locations = await Promise.all(
    (locationsResult.ok ? locationsResult.data : []).map(async (location) => {
      const rooms = await listRooms(location.id);

      return { id: location.id, name: location.name, rooms: rooms.ok ? rooms.data : [] };
    }),
  );

  const result = await listItems(toApiParams(values, selectedCategory?.traits ?? []));

  const filtering = hasActiveFilters(values);
  const view = values.view === "grid" ? "grid" : "list";

  const newItemParams = new URLSearchParams();
  if (values.locationId) newItemParams.set("locationId", values.locationId);
  if (values.roomId) newItemParams.set("roomId", values.roomId);
  const newItemQuery = newItemParams.toString();
  const newItemHref = `/items/new${newItemQuery ? `?${newItemQuery}` : ""}`;

  const hits = result.ok ? result.data.items : [];
  const nextCursor = result.ok ? result.data.nextCursor : undefined;
  const narrowedByPriceOrDate = PRICE_AND_DATE_FILTERS.some((key) => values[key] !== undefined);

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <div className="mt-4">
        <Link
          href={newItemHref}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("addButton")}
        </Link>
        <Link
          href="/items/batch"
          className="ml-2 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("batchButton")}
        </Link>
        {/* A plain link: the session cookie rides along and the server's
            Content-Disposition makes the browser save the file. */}
        <a
          href="/api/v1/items/export"
          download
          className="ml-2 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("exportButton")}
        </a>
        <Link
          href="/items/import"
          className="ml-2 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
        >
          {t("importButton")}
        </Link>
      </div>

      <FilterPanel
        values={values}
        open={filtering && values.q === undefined}
        lookups={{
          categories,
          locations,
          people: peopleResult.ok ? peopleResult.data : [],
          tags: tagsResult.ok ? tagsResult.data : [],
        }}
      />

      <section aria-labelledby="results" className="mt-8">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="results" className="text-sm font-medium uppercase tracking-wide text-muted">
            {filtering ? ts("results") : ts("allItems")}
          </h2>

          <div className="flex items-center gap-1 text-sm" role="group" aria-label={ts("view")}>
            <ViewLink href={hrefWith(values, { view: undefined })} current={view === "list"}>
              {ts("viewList")}
            </ViewLink>
            <ViewLink href={hrefWith(values, { view: "grid" })} current={view === "grid"}>
              {ts("viewGrid")}
            </ViewLink>
          </div>
        </div>

        {!result.ok && <Notice tone="warning" title={result.detail} />}

        {result.ok && hits.length === 0 && (
          <Notice title={filtering ? ts("noMatches") : t("empty")}>
            {filtering && (
              <>
                <p>{ts("noMatchesHint")}</p>
                <Link
                  href="/items"
                  className="mt-2 inline-flex min-h-11 items-center underline underline-offset-4"
                >
                  {ts("clear")}
                </Link>
              </>
            )}
          </Notice>
        )}

        {hits.length > 0 && narrowedByPriceOrDate && (
          <p className="mt-3 text-sm text-muted">{ts("partialDataHint")}</p>
        )}

        {hits.length > 0 && view === "list" && (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {hits.map((hit) => (
              <li key={hit.item.id}>
                <Link
                  href={`/items/${hit.item.id}`}
                  className="flex min-h-11 items-center gap-3 px-4 py-3 hover:bg-border/40"
                >
                  <Thumb hit={hit} size={48} />
                  <HitText hit={hit} statusLabel={t(STATUS_KEYS[hit.item.status])} />
                </Link>
              </li>
            ))}
          </ul>
        )}

        {hits.length > 0 && view === "grid" && (
          <ul className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
            {hits.map((hit) => (
              <li key={hit.item.id}>
                <Link
                  href={`/items/${hit.item.id}`}
                  className="block overflow-hidden rounded-lg border border-border hover:bg-border/40"
                >
                  <Thumb hit={hit} size={THUMB_PX} fill />
                  <div className="p-3">
                    <HitText hit={hit} statusLabel={t(STATUS_KEYS[hit.item.status])} />
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}

        {nextCursor && (
          <Link
            href={hrefWith(values, { cursor: nextCursor })}
            className="mt-4 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            {ts("nextPage")}
          </Link>
        )}
      </section>
    </AppShell>
  );
}

function ViewLink({
  href,
  current,
  children,
}: {
  href: string;
  current: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      aria-current={current ? "true" : undefined}
      className={`inline-flex min-h-11 items-center rounded-md px-3 hover:bg-border/60 ${
        current ? "bg-border/60 font-medium" : ""
      }`}
    >
      {children}
    </Link>
  );
}

function Thumb({ hit, size, fill = false }: { hit: ItemHit; size: number; fill?: boolean }) {
  const shape = fill ? "aspect-square w-full" : "size-12 shrink-0 rounded-md";

  if (!hit.thumbUrl) {
    return (
      <div
        aria-hidden="true"
        className={`flex items-center justify-center bg-border/40 text-lg text-muted ${shape}`}
      >
        {hit.item.name.slice(0, 1).toUpperCase()}
      </div>
    );
  }

  return (
    <Image
      src={hit.thumbUrl}
      alt=""
      unoptimized
      width={size}
      height={size}
      className={`object-cover ${shape}`}
    />
  );
}

/** Name, then where it is: «Box › Room · Location». */
function HitText({ hit, statusLabel }: { hit: ItemHit; statusLabel: string }) {
  const { placement } = hit;
  const where = [
    ...placement.containerPath.map((c) => c.name),
    placement.room?.name,
    placement.location.name,
  ]
    .filter(Boolean)
    .join(" › ");

  return (
    <div className="min-w-0 flex-1">
      <p className="truncate text-sm font-medium">{hit.item.name}</p>
      <p className="truncate text-sm text-muted">{where}</p>
      <p className="truncate text-xs text-muted">{statusLabel}</p>
    </div>
  );
}
