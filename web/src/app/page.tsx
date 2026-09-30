import Link from "next/link";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listFamilyMembers } from "@/lib/api/family";
import { requireMe } from "@/lib/api/session";

/**
 * The inventory, which in M1 is empty on purpose.
 *
 * The milestone's acceptance sentence is that two Google accounts share one
 * family and both see the same (empty) inventory — so this page's job is to
 * make the sharing visible and be honest that items arrive in M3. An empty
 * inventory is a normal state of this application, not a failure, and it is
 * rendered as one.
 */
export default async function Home() {
  const me = await requireMe("/");
  const members = await listFamilyMembers();

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

        <div className="mt-3">
          <Notice title="Nothing here yet">
            Items arrive in M3. Until then, this family exists, everyone in it sees the same (empty)
            inventory, and you can record the people you will be tracking things for.
          </Notice>
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
