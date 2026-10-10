import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { AddFieldForm } from "@/app/settings/taxonomy/[categoryId]/add-field-form";
import { archive } from "@/app/settings/taxonomy/[categoryId]/actions";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getTaxonomy, type Category } from "@/lib/api/taxonomy";
import { requireMe } from "@/lib/api/session";

const DATA_TYPE_KEYS = {
  text: "typeText",
  number: "typeNumber",
  integer: "typeInteger",
  bool: "typeBool",
  date: "typeDate",
  enum: "typeEnum",
  size: "typeSize",
} as const;

/** Depth-first search: the tree is a handful of levels deep at most. */
function findCategory(categories: Category[], id: string): Category | undefined {
  for (const category of categories) {
    if (category.id === id) {
      return category;
    }

    const found = findCategory(category.children, id);
    if (found) {
      return found;
    }
  }

  return undefined;
}

export default async function CategoryDetailPage(
  props: PageProps<"/settings/taxonomy/[categoryId]">,
) {
  const { categoryId } = await props.params;

  const me = await requireMe(`/settings/taxonomy/${categoryId}`);
  const t = await getTranslations("CategoryDetail");

  const taxonomy = await getTaxonomy();
  if (!taxonomy.ok) {
    throw new Error(taxonomy.detail);
  }

  const category = findCategory(taxonomy.data.categories, categoryId);
  if (!category) {
    notFound();
  }

  const archiveInThisCategory = archive.bind(null, categoryId);

  return (
    <AppShell me={me}>
      <Link href="/settings/taxonomy" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{category.name}</h1>

      <section aria-labelledby="fields" className="mt-10">
        <h2 id="fields" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("traitsHeading")}
        </h2>

        {category.traits.length === 0 && (
          <div className="mt-3">
            <Notice>{t("noTraits")}</Notice>
          </div>
        )}

        {category.traits.length > 0 && (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {category.traits.map((trait) => (
              <li
                key={trait.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{trait.label}</p>
                  <p className="truncate text-sm text-muted">{t(DATA_TYPE_KEYS[trait.dataType])}</p>
                </div>

                {!trait.isBuiltin && (
                  <form action={archiveInThisCategory}>
                    <input type="hidden" name="traitDefId" value={trait.id} />
                    <button
                      type="submit"
                      className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                    >
                      {t("archiveButton")}
                    </button>
                  </form>
                )}
              </li>
            ))}
          </ul>
        )}

        <h3 className="mt-6 text-sm font-medium">{t("addFieldHeading")}</h3>
        <AddFieldForm categoryId={categoryId} />
      </section>
    </AppShell>
  );
}
