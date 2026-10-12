"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { Input } from "@/components/ui/input";

const DEBOUNCE_MS = 250;

/**
 * The search box. Typing updates the URL after a short pause, and the server
 * component above re-renders the results — the URL, not client state, is the
 * source of truth, so a search is linkable and survives a refresh.
 *
 * It sits inside the filter form as well, so pressing Enter (or running
 * without JavaScript) submits the same `q` the debounce would have written.
 */
export function SearchBar({ initial }: { initial: string }) {
  const t = useTranslations("Search");
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [value, setValue] = useState(initial);

  useEffect(() => {
    if (value === initial) return;

    const timer = setTimeout(() => {
      const next = new URLSearchParams(searchParams.toString());

      if (value.trim() === "") next.delete("q");
      else next.set("q", value);

      // A new query starts a new list; the old cursor belongs to the old one.
      next.delete("cursor");
      next.delete("sort");

      const query = next.toString();
      router.replace(query === "" ? pathname : `${pathname}?${query}`, { scroll: false });
    }, DEBOUNCE_MS);

    return () => clearTimeout(timer);
  }, [value, initial, searchParams, pathname, router]);

  return (
    <Input
      type="search"
      name="q"
      value={value}
      onChange={(event) => setValue(event.target.value)}
      placeholder={t("placeholder")}
      aria-label={t("label")}
      maxLength={200}
      autoComplete="off"
      enterKeyHint="search"
    />
  );
}
