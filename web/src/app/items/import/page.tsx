import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ImportWizard } from "@/app/items/import/import-wizard";
import { AppShell } from "@/components/app-shell";
import { listLocations } from "@/lib/api/places";
import { requireMe } from "@/lib/api/session";
import { flattenCategories, getTaxonomy } from "@/lib/api/taxonomy";

export default async function ImportItemsPage() {
  const me = await requireMe("/items/import");
  const t = await getTranslations("Import");

  const [taxonomy, locationsResult] = await Promise.all([getTaxonomy(), listLocations()]);

  if (!taxonomy.ok) {
    throw new Error(taxonomy.detail);
  }

  const locations = (locationsResult.ok ? locationsResult.data : []).map((l) => ({
    id: l.id,
    name: l.name,
    rooms: [],
  }));

  return (
    <AppShell me={me}>
      <Link href="/items" className="text-sm text-muted hover:underline">
        ← {t("backLink")}
      </Link>

      <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{t("heading")}</h1>

      <ImportWizard
        categories={flattenCategories(taxonomy.data.categories)}
        locations={locations}
      />
    </AppShell>
  );
}
