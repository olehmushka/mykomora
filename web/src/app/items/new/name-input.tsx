"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { findSimilarAction } from "@/app/items/new/actions";
import { Input } from "@/components/ui/input";
import type { SimilarItem } from "@/lib/api/items";

/**
 * The item name, plus a gentle "you may already have this" list once the
 * person leaves the field. It never blocks saving: a family can own two
 * identical towels.
 */
export function NameInput() {
  const t = useTranslations("ItemForm");
  const [similar, setSimilar] = useState<SimilarItem[]>([]);

  async function check(name: string) {
    try {
      setSimilar(await findSimilarAction(name));
    } catch {
      setSimilar([]);
    }
  }

  return (
    <>
      <Input
        id="item-name"
        name="name"
        required
        maxLength={120}
        onBlur={(event) => void check(event.target.value)}
        onChange={() => setSimilar([])}
      />

      {similar.length > 0 && (
        <div className="mt-2 rounded-md border border-border px-3 py-2 text-sm" role="status">
          <p className="text-muted">{t("similarHeading")}</p>
          <ul className="mt-1">
            {similar.map((item) => (
              <li key={item.id}>
                <Link
                  href={`/items/${item.id}`}
                  target="_blank"
                  className="inline-flex min-h-11 items-center underline underline-offset-4"
                >
                  {item.name}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  );
}
