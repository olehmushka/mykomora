"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";

import { shoppingListText, type ShoppingEntry } from "@/lib/shopping-text";

const buttonClass =
  "inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60";

/** Copy the list to the clipboard or save it as a .txt file. */
export function ShoppingListExport({ entries }: { entries: ShoppingEntry[] }) {
  const t = useTranslations("ShoppingList");
  const [copied, setCopied] = useState(false);
  const text = shoppingListText(entries);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access can be refused (insecure origin, permissions); the
      // download button below still works.
      setCopied(false);
    }
  }

  return (
    <div className="mt-4 flex flex-wrap items-center gap-3">
      <button type="button" onClick={copy} className={buttonClass}>
        {copied ? t("copied") : t("copy")}
      </button>
      <a
        href={`data:text/plain;charset=utf-8,${encodeURIComponent(text)}`}
        download="shopping-list.txt"
        className={buttonClass}
      >
        {t("download")}
      </a>
    </div>
  );
}
