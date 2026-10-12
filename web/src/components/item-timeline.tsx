import { getTranslations } from "next-intl/server";

import type { FamilyMember } from "@/lib/api/family";
import type { ItemEvent } from "@/lib/api/items";

type T = Awaited<ReturnType<typeof getTranslations>>;

function str(payload: ItemEvent["payload"], key: string): string | undefined {
  const value = payload[key];

  return typeof value === "string" ? value : undefined;
}

/** One localised sentence per event, built from its type and payload. */
function describe(event: ItemEvent, t: T, status: (value: string) => string): string {
  const p = event.payload;

  switch (event.type) {
    case "created":
      return t("created");
    case "updated":
      return t("updated");
    case "moved":
      return t("moved");
    case "archived":
      return t("archived");
    case "quantity_changed":
      return t("quantity_changed");
    case "photo_added":
      return t("photo_added");
    case "photo_removed":
      return t("photo_removed");
    case "attachment_added":
      return t("attachment_added");
    case "attachment_removed":
      return t("attachment_removed");
    case "status_changed": {
      const from = str(p, "from");
      const to = str(p, "to");

      return from && to
        ? t("status_changed_from_to", { from: status(from), to: status(to) })
        : t("status_changed");
    }
    case "lent": {
      const name = str(p, "borrowerName");
      const due = str(p, "dueAt");

      return name
        ? due
          ? t("lent_to_due", { name, date: due })
          : t("lent_to", { name })
        : due
          ? t("lent_due", { date: due })
          : t("lent");
    }
    case "returned":
      return t("returned");
    default:
      return event.type;
  }
}

/**
 * The item's history, newest first (M9): who did what, when, in words. The
 * actor is resolved from the family's members; an unknown or removed member
 * falls back to a neutral placeholder rather than hiding the event.
 */
export async function ItemTimeline({
  events,
  members,
  locale,
  statusLabel,
}: {
  events: ItemEvent[];
  members: FamilyMember[];
  locale: string;
  statusLabel: (value: string) => string;
}) {
  const t = await getTranslations("Timeline");
  const byId = new Map(members.map((m) => [m.userId, m]));
  const when = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });

  return (
    <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
      {events.map((event) => {
        const actor = byId.get(event.actorUserId);
        const name = actor?.displayName ?? t("someone");

        return (
          <li key={event.id} className="flex items-start gap-3 px-4 py-3 text-sm">
            <Avatar name={name} url={actor?.avatarUrl} />
            <div className="min-w-0">
              <p>{describe(event, t, statusLabel)}</p>
              <p className="text-muted">
                {name} ·{" "}
                <time dateTime={event.createdAt}>{when.format(new Date(event.createdAt))}</time>
              </p>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function Avatar({ name, url }: { name: string; url?: string }) {
  if (url) {
    return (
      <span
        aria-hidden="true"
        className="size-8 shrink-0 rounded-full bg-cover bg-center"
        style={{ backgroundImage: `url(${JSON.stringify(url)})` }}
      />
    );
  }

  return (
    <span
      aria-hidden="true"
      className="inline-flex size-8 shrink-0 items-center justify-center rounded-full border border-border text-xs font-medium"
    >
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}
