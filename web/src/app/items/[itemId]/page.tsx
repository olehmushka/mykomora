import Link from "next/link";
import { notFound } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import {
  archive,
  attachTag,
  detachTag,
  duplicate,
  markReturned,
} from "@/app/items/[itemId]/actions";
import { AttachmentsSection } from "@/app/items/[itemId]/attachments-section";
import { MoveForm } from "@/app/items/[itemId]/move-form";
import { PhotoGallery } from "@/app/items/[itemId]/photo-gallery";
import { SeasonFlip } from "@/app/items/[itemId]/season-flip";
import { LendForm } from "@/app/items/[itemId]/lend-form";
import { StatusForm } from "@/app/items/[itemId]/status-form";
import { AppShell } from "@/components/app-shell";
import { ItemTimeline } from "@/components/item-timeline";
import { Notice } from "@/components/notice";
import { QuantityStepper } from "@/components/quantity-stepper";
import { listItemAttachments } from "@/lib/api/attachments";
import {
  getItem,
  listItemContents,
  listItemEvents,
  listItemTags,
  listItems,
  type ItemStatus,
} from "@/lib/api/items";
import { listFamilyMembers } from "@/lib/api/family";
import { listItemLoans } from "@/lib/api/loans";
import { listPeople } from "@/lib/api/people";
import { listItemPhotos } from "@/lib/api/photos";
import { listLocations, listRooms } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";
import { getTaxonomy, type Category } from "@/lib/api/taxonomy";
import { listTags } from "@/lib/api/tags";

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

function findCategoryName(categories: Category[], id: string): string | undefined {
  for (const category of categories) {
    if (category.id === id) return category.name;

    const found = findCategoryName(category.children, id);
    if (found) return found;
  }

  return undefined;
}

export default async function ItemDetailPage(props: PageProps<"/items/[itemId]">) {
  const { itemId } = await props.params;

  const me = await requireMe(`/items/${itemId}`);
  const t = await getTranslations("ItemDetail");
  const tLoans = await getTranslations("Loans");
  const locale = await getLocale();

  const itemResult = await getItem(itemId);
  if (!itemResult.ok) {
    if (itemResult.reason === "missing") notFound();
    throw new Error(itemResult.detail);
  }

  const item = itemResult.data;

  const [
    taxonomy,
    contentsResult,
    eventsResult,
    itemTagsResult,
    allTagsResult,
    locationsResult,
    rootsResult,
    parentResult,
    photosResult,
    attachmentsResult,
    peopleResult,
    membersResult,
    loansResult,
  ] = await Promise.all([
    getTaxonomy(),
    item.isContainer ? listItemContents(itemId) : Promise.resolve(undefined),
    listItemEvents(itemId),
    listItemTags(itemId),
    listTags(),
    listLocations(),
    listItems({}),
    item.parentItemId ? getItem(item.parentItemId) : Promise.resolve(undefined),
    listItemPhotos(itemId),
    listItemAttachments(itemId),
    listPeople(),
    listFamilyMembers(),
    listItemLoans(itemId),
  ]);

  const categoryName = taxonomy.ok
    ? findCategoryName(taxonomy.data.categories, item.categoryId)
    : undefined;

  const locations = locationsResult.ok ? locationsResult.data : [];
  const locationsWithRooms = await Promise.all(
    locations.map(async (location) => {
      const rooms = await listRooms(location.id);

      return { id: location.id, name: location.name, rooms: rooms.ok ? rooms.data : [] };
    }),
  );

  const containers = (rootsResult.ok ? rootsResult.data.items.map((hit) => hit.item) : [])
    .filter((candidate) => candidate.isContainer && candidate.id !== item.id)
    .map((candidate) => ({ id: candidate.id, name: candidate.name }));

  const itemTagIds = new Set((itemTagsResult.ok ? itemTagsResult.data : []).map((tag) => tag.id));
  const availableTags = (allTagsResult.ok ? allTagsResult.data : []).filter(
    (tag) => !itemTagIds.has(tag.id),
  );

  const openLoan = loansResult.ok ? loansResult.data.find((loan) => !loan.returnedAt) : undefined;
  const dateOnly = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" });
  const returnThisItem = markReturned.bind(null, itemId);

  const detachInThisItem = detachTag.bind(null, itemId);
  const attachInThisItem = attachTag.bind(null, itemId);
  const archiveInThisItem = archive.bind(null, itemId);
  const duplicateThisItem = duplicate.bind(null, itemId);

  return (
    <AppShell me={me}>
      <Link href="/items" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      {item.parentItemId && parentResult?.ok && (
        <p className="mt-1 text-sm text-muted">
          {t("insideContainer")}{" "}
          <Link href={`/items/${item.parentItemId}`} className="underline underline-offset-4">
            {parentResult.data.name}
          </Link>
        </p>
      )}

      <div className="mt-1 flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{item.name}</h1>
        {item.isContainer && (
          <span className="rounded-full border border-border px-2 py-0.5 text-xs text-muted">
            {t("containerBadge")}
          </span>
        )}
      </div>

      <dl className="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
        {categoryName && (
          <div>
            <dt className="text-muted">{t("categoryLabel")}</dt>
            <dd>{categoryName}</dd>
          </div>
        )}
        <div>
          <dt className="text-muted">{t("quantityLabel")}</dt>
          <dd>
            <QuantityStepper itemId={itemId} quantity={item.quantity} unit={item.unit} />
          </dd>
        </div>
        {item.effectiveExpiresAt && (
          <div>
            <dt className="text-muted">{t("expiresLabel")}</dt>
            <dd>
              {new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" }).format(
                new Date(item.effectiveExpiresAt),
              )}
              {item.openedAt && item.paoMonths && item.effectiveExpiresAt !== item.expiresAt && (
                <span className="block text-xs text-muted">
                  {t("paoHint", { months: item.paoMonths })}
                </span>
              )}
            </dd>
          </div>
        )}
        {item.brand && (
          <div>
            <dt className="text-muted">{t("brandLabel")}</dt>
            <dd>{item.brand}</dd>
          </div>
        )}
        {item.sizeValue && (
          <div>
            <dt className="text-muted">{t("sizeLabel")}</dt>
            <dd>{item.sizeValue}</dd>
          </div>
        )}
        {item.notes && (
          <div className="col-span-2 sm:col-span-3">
            <dt className="text-muted">{t("notesLabel")}</dt>
            <dd>{item.notes}</dd>
          </div>
        )}
      </dl>

      <section aria-labelledby="photos" className="mt-10">
        <h2 id="photos" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("photosHeading")}
        </h2>

        {photosResult.ok ? (
          <PhotoGallery itemId={itemId} photos={photosResult.data} />
        ) : (
          <div className="mt-3">
            <Notice tone="warning" title={photosResult.detail} />
          </div>
        )}
      </section>

      <section aria-labelledby="attachments" className="mt-10">
        <h2 id="attachments" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("attachmentsHeading")}
        </h2>

        {attachmentsResult.ok ? (
          <AttachmentsSection itemId={itemId} attachments={attachmentsResult.data} />
        ) : (
          <div className="mt-3">
            <Notice tone="warning" title={attachmentsResult.detail} />
          </div>
        )}
      </section>

      <section aria-labelledby="status" className="mt-10">
        <h2 id="status" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("statusHeading")}
        </h2>

        {item.archivedAt ? (
          <div className="mt-3">
            <Notice title={t(STATUS_KEYS[item.status])}>{t("archivedNotice")}</Notice>
          </div>
        ) : (
          <>
            <p className="mt-3 text-sm">{t(STATUS_KEYS[item.status])}</p>

            {item.status === "lent" && openLoan && (
              <div className="mt-3">
                <Notice
                  tone={openLoan.overdue ? "warning" : "neutral"}
                  title={tLoans("lentBanner", { name: openLoan.borrowerName ?? "—" })}
                >
                  {openLoan.dueAt
                    ? openLoan.overdue
                      ? tLoans("overdueSince", { date: dateOnly.format(new Date(openLoan.dueAt)) })
                      : tLoans("dueOn", { date: dateOnly.format(new Date(openLoan.dueAt)) })
                    : tLoans("noDueDate")}
                  {openLoan.note ? ` · ${openLoan.note}` : ""}
                </Notice>
                <form action={returnThisItem} className="mt-3">
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
                  >
                    {tLoans("returnButton")}
                  </button>
                </form>
              </div>
            )}

            {item.status !== "lent" && (
              <details className="mt-3">
                <summary className="cursor-pointer text-sm font-medium">
                  {tLoans("lendHeading")}
                </summary>
                <LendForm
                  itemId={itemId}
                  people={(peopleResult.ok ? peopleResult.data : []).map((p) => ({
                    id: p.id,
                    name: p.name,
                  }))}
                />
              </details>
            )}

            <StatusForm itemId={itemId} current={item.status} />

            <form action={duplicateThisItem} className="mt-3">
              <button
                type="submit"
                className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
              >
                {t("duplicateButton")}
              </button>
            </form>

            <form action={archiveInThisItem} className="mt-3">
              <button
                type="submit"
                className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
              >
                {t("archiveButton")}
              </button>
            </form>
          </>
        )}
      </section>

      {!item.archivedAt && (
        <section aria-labelledby="move" className="mt-10">
          <h2 id="move" className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("moveHeading")}
          </h2>
          <MoveForm itemId={itemId} locations={locationsWithRooms} containers={containers} />
        </section>
      )}

      {item.isContainer && (
        <section aria-labelledby="contents" className="mt-10">
          <h2 id="contents" className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("contentsHeading")}
          </h2>

          {contentsResult && !contentsResult.ok && (
            <div className="mt-3">
              <Notice tone="warning" title={contentsResult.detail} />
            </div>
          )}

          {contentsResult?.ok && contentsResult.data.length === 0 && (
            <div className="mt-3">
              <Notice>{t("noContents")}</Notice>
            </div>
          )}

          {contentsResult?.ok && contentsResult.data.length > 0 && <SeasonFlip itemId={itemId} />}

          {contentsResult?.ok && contentsResult.data.length > 0 && (
            <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
              {contentsResult.data.map((child) => (
                <li key={child.id} className="flex items-center justify-between gap-3 px-4 py-3">
                  <Link
                    href={`/items/${child.id}`}
                    className="truncate text-sm font-medium hover:underline"
                  >
                    {child.name}
                  </Link>
                  <span className="text-sm text-muted">{t(STATUS_KEYS[child.status])}</span>
                </li>
              ))}
            </ul>
          )}

          <Link
            href={`/items/new?parentItemId=${encodeURIComponent(itemId)}`}
            className="mt-3 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            {t("addInsideButton")}
          </Link>
          <Link
            href={`/items/batch?parentItemId=${encodeURIComponent(itemId)}`}
            className="ml-2 mt-3 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            {t("addManyInsideButton")}
          </Link>
        </section>
      )}

      <section aria-labelledby="tags" className="mt-10">
        <h2 id="tags" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("tagsHeading")}
        </h2>

        <div className="mt-3 flex flex-wrap gap-2">
          {(itemTagsResult.ok ? itemTagsResult.data : []).map((tag) => (
            <form key={tag.id} action={detachInThisItem}>
              <input type="hidden" name="tagId" value={tag.id} />
              <button
                type="submit"
                className="inline-flex min-h-11 items-center gap-1 rounded-full border border-border px-3 text-sm hover:bg-border/60"
              >
                {tag.name} ×
              </button>
            </form>
          ))}

          {itemTagsResult.ok && itemTagsResult.data.length === 0 && (
            <p className="text-sm text-muted">{t("noTags")}</p>
          )}
        </div>

        {availableTags.length > 0 && (
          <form action={attachInThisItem} className="mt-3 flex flex-wrap items-end gap-3">
            <select
              name="tagId"
              defaultValue=""
              className="mt-1 min-h-11 rounded-md border border-border bg-transparent px-3 text-sm"
            >
              <option value="" disabled>
                {t("chooseTag")}
              </option>
              {availableTags.map((tag) => (
                <option key={tag.id} value={tag.id}>
                  {tag.name}
                </option>
              ))}
            </select>
            <button
              type="submit"
              className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
            >
              {t("attachTagButton")}
            </button>
          </form>
        )}

        <p className="mt-3 text-sm">
          <Link href="/settings/tags" className="underline underline-offset-4">
            {t("manageTagsLink")}
          </Link>
        </p>
      </section>

      <section aria-labelledby="events" className="mt-10">
        <h2 id="events" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("eventsHeading")}
        </h2>

        {!eventsResult.ok && (
          <div className="mt-3">
            <Notice tone="warning" title={eventsResult.detail} />
          </div>
        )}

        {eventsResult.ok && eventsResult.data.length > 0 && (
          <ItemTimeline
            events={eventsResult.data}
            members={membersResult.ok ? membersResult.data : []}
            locale={locale}
            statusLabel={(value) => t(`status.${value}`)}
          />
        )}
      </section>
    </AppShell>
  );
}
