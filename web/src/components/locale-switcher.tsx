"use client";

import { useTransition } from "react";
import { useLocale, useTranslations } from "next-intl";

import { setLocale } from "@/app/actions";
import { locales } from "@/i18n/locale";

/**
 * Changes the cookie every later render resolves from (and, if signed in,
 * the account's stored preference) — see src/i18n/locale.ts and
 * app/actions.ts's setLocale for the two-destination write this triggers.
 */
export function LocaleSwitcher() {
  const locale = useLocale();
  const t = useTranslations("LocaleSwitcher");
  const [pending, startTransition] = useTransition();

  return (
    <label className="inline-flex items-center gap-1 text-sm">
      <span className="sr-only">{t("label")}</span>
      <select
        value={locale}
        disabled={pending}
        onChange={(event) => startTransition(() => setLocale(event.target.value))}
        className="min-h-11 rounded-md border border-border bg-transparent px-2 text-sm disabled:opacity-60"
      >
        {locales.map((value) => (
          <option key={value} value={value}>
            {value === "en" ? t("english") : t("ukrainian")}
          </option>
        ))}
      </select>
    </label>
  );
}
