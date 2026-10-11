import Link from "next/link";

import { AppShell } from "@/components/app-shell";
import { listFamilyMembers } from "@/lib/api/family";
import { listItems } from "@/lib/api/items";
import { requireMe } from "@/lib/api/session";

/**
 * The inventory landing page.
 *
 * As of M3, items are real: this page's job is a quick count and the two
 * doors into the catalogue, not the catalogue itself — that's /items.
 */
export default async function Home() {
  const me = await requireMe("/");
  const [members, items] = await Promise.all([listFamilyMembers(), listItems()]);

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{me.family.name}</h1>
      <p className="mt-2 text-sm text-muted">
        Signed in as {me.user.displayName}
        {me.role === "owner" && " · owner"}
      </p>

      <section aria-labelledby="inventory" className="mt-10">
        <h2 id="inventory" className="text-sm font-medium uppercase tracking-wide text-muted">
          Inventory
        </h2>

        <p className="mt-3 text-sm">
          {items.ok
            ? `${items.data.length} ${items.data.length === 1 ? "item" : "items"} recorded at the top level.`
            : "The item count is unavailable right now."}
        </p>

        <div className="mt-3 flex flex-wrap gap-3">
          <Link
            href="/items"
            className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            Browse items
          </Link>
          <Link
            href="/items/new"
            className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
          >
            Add item
          </Link>
        </div>
      </section>

      <section aria-labelledby="sharing" className="mt-10">
        <h2 id="sharing" className="text-sm font-medium uppercase tracking-wide text-muted">
          Sharing
        </h2>

        <p className="mt-3 text-sm">
          {members.ok
            ? `${members.data.length} ${members.data.length === 1 ? "person has" : "people have"} access to this family.`
            : "The member list is unavailable right now."}{" "}
          <Link href="/settings/family" className="underline underline-offset-4">
            Family settings
          </Link>
        </p>
      </section>
    </AppShell>
  );
}
