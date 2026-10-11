import { getTranslations } from "next-intl/server";

import { AddTagForm } from "@/app/settings/tags/add-tag-form";
import { remove, rename } from "@/app/settings/tags/actions";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { listTags } from "@/lib/api/tags";
import { requireMe } from "@/lib/api/session";

export const metadata = { title: "Tags — mykomora" };

export default async function TagsPage() {
  const me = await requireMe("/settings/tags");
  const t = await getTranslations("Tags");
  const tags = await listTags();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <section aria-labelledby="add" className="mt-10">
        <h2 id="add" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("addHeading")}
        </h2>
        <AddTagForm />
      </section>

      <section aria-labelledby="list" className="mt-10">
        {!tags.ok && <Notice tone="warning" title={tags.detail} />}

        {tags.ok && tags.data.length === 0 && <Notice>{t("empty")}</Notice>}

        {tags.ok && tags.data.length > 0 && (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {tags.data.map((tag) => (
              <li key={tag.id} className="flex flex-wrap items-end gap-3 px-4 py-3">
                <form action={rename} className="flex flex-1 flex-wrap items-end gap-3">
                  <input type="hidden" name="tagId" value={tag.id} />
                  <div className="flex-1">
                    <label htmlFor={`tag-name-${tag.id}`} className="sr-only">
                      {t("nameLabel")}
                    </label>
                    <input
                      id={`tag-name-${tag.id}`}
                      name="name"
                      defaultValue={tag.name}
                      required
                      maxLength={60}
                      className="min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
                    />
                  </div>
                  <input
                    aria-label={t("colorLabel")}
                    name="color"
                    defaultValue={tag.color ?? ""}
                    placeholder="#4a7c59"
                    className="min-h-11 w-32 rounded-md border border-border bg-transparent px-3 text-sm"
                  />
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    {t("saveButton")}
                  </button>
                </form>

                <form action={remove}>
                  <input type="hidden" name="tagId" value={tag.id} />
                  <button
                    type="submit"
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                  >
                    {t("deleteButton")}
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
