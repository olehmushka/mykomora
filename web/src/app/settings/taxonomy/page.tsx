import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { AddCategoryForm } from "@/app/settings/taxonomy/add-category-form";
import { CategoryNode } from "@/app/settings/taxonomy/category-node";
import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { getTaxonomy } from "@/lib/api/taxonomy";
import { requireMe } from "@/lib/api/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Meta");

  return { title: t("categories") };
}

/**
 * The category tree: built-in ∪ this family's own, each with its resolved
 * trait schema (SPEC 4, "Taxonomy"). Labels come back already localised —
 * this page renders what GET /taxonomy sends, nothing more.
 */
export default async function TaxonomyPage() {
  const me = await requireMe("/settings/taxonomy");
  const t = await getTranslations("Taxonomy");
  const taxonomy = await getTaxonomy();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <section aria-labelledby="add" className="mt-10">
        <h2 id="add" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("addCategoryHeading")}
        </h2>
        <AddCategoryForm />
      </section>

      <section aria-labelledby="list" className="mt-10">
        {!taxonomy.ok && <Notice tone="warning" title={taxonomy.detail} />}

        {taxonomy.ok && taxonomy.data.categories.length === 0 && <Notice>{t("empty")}</Notice>}

        {taxonomy.ok && taxonomy.data.categories.length > 0 && (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {taxonomy.data.categories.map((category) => (
              <CategoryNode key={category.id} category={category} />
            ))}
          </ul>
        )}
      </section>
    </AppShell>
  );
}
