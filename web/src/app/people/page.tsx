import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { archive } from "@/app/people/actions";
import { AddPersonForm } from "@/app/people/add-person-form";
import { listPeople } from "@/lib/api/people";
import { requireMe } from "@/lib/api/session";

export const metadata = { title: "People — mykomora" };

/**
 * The people a family tracks things for.
 *
 * Not the same as the members who can sign in: the second-priority job of this
 * application is knowing what fits the children, and children do not have
 * Google accounts (SPEC 3).
 */
export default async function PeoplePage() {
  const me = await requireMe("/people");
  const people = await listPeople();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">People</h1>
      <p className="mt-2 text-sm text-muted">
        Anyone this family keeps things for — a child, a grandparent. They do not need an account,
        and from M3 items can be recorded against them.
      </p>

      <section aria-labelledby="add" className="mt-10">
        <h2 id="add" className="text-sm font-medium uppercase tracking-wide text-muted">
          Add someone
        </h2>
        <AddPersonForm />
      </section>

      <section aria-labelledby="list" className="mt-10">
        <h2 id="list" className="text-sm font-medium uppercase tracking-wide text-muted">
          In this family
        </h2>

        {!people.ok && (
          <div className="mt-3">
            <Notice tone="warning" title="The list is unavailable">
              {people.detail}
            </Notice>
          </div>
        )}

        {people.ok && people.data.length === 0 && (
          <div className="mt-3">
            <Notice title="Nobody recorded yet">
              Add the children whose sizes you want to track, and anyone else you buy for.
            </Notice>
          </div>
        )}

        {people.ok && people.data.length > 0 && (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {people.data.map((person) => (
              <li
                key={person.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{person.name}</p>
                  <p className="truncate text-sm text-muted">
                    {person.isChild ? "Child" : "Adult"}
                    {person.birthdate ? ` · born ${person.birthdate}` : ""}
                  </p>
                </div>

                <form action={archive}>
                  <input type="hidden" name="personId" value={person.id} />
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    Archive
                  </button>
                </form>
              </li>
            ))}
          </ul>
        )}
      </section>
    </AppShell>
  );
}
