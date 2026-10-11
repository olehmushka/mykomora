import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { archive } from "@/app/settings/taxonomy/actions";
import type { Category } from "@/lib/api/taxonomy";

/** One category and its children, nested — the tree is a handful of levels deep at most. */
export async function CategoryNode({ category }: { category: Category }) {
  const t = await getTranslations("Taxonomy");

  return (
    <li>
      <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div className="min-w-0">
          <span className="text-sm font-medium">{category.name}</span>{" "}
          <span className="text-xs text-muted">
            {category.isBuiltin ? t("builtinBadge") : t("customBadge")}
          </span>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href={`/settings/taxonomy/${category.id}`}
            className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
          >
            {t("viewLink", { count: category.traits.length })}
          </Link>

          {!category.isBuiltin && (
            <form action={archive}>
              <input type="hidden" name="categoryId" value={category.id} />
              <button
                type="submit"
                className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
              >
                {t("archiveButton")}
              </button>
            </form>
          )}
        </div>
      </div>

      {category.children.length > 0 && (
        <ul className="ml-4 divide-y divide-border border-l border-border pl-4">
          {category.children.map((child) => (
            <CategoryNode key={child.id} category={child} />
          ))}
        </ul>
      )}
    </li>
  );
}
