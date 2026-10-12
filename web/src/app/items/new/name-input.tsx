"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { lookupBarcodeAction } from "@/app/items/lookup-action";
import { findSimilarAction } from "@/app/items/new/actions";
import { BarcodeScanner } from "@/components/barcode-scanner";
import { Input } from "@/components/ui/input";
import type { SimilarItem } from "@/lib/api/items";

/**
 * The item name, plus a gentle "you may already have this" list once the
 * person leaves the field. It never blocks saving: a family can own two
 * identical towels.
 */
export function NameInput() {
  const t = useTranslations("ItemForm");
  const tScan = useTranslations("Scanner");
  const [similar, setSimilar] = useState<SimilarItem[]>([]);
  const [scanning, setScanning] = useState(false);
  const [note, setNote] = useState("");
  const nameRef = useRef<HTMLInputElement>(null);

  // Fills the name and, when the catalogue knows it, the brand field
  // elsewhere in the same form. Both stay editable.
  async function onScan(code: string) {
    setNote(tScan("looking", { code }));

    const product = await lookupBarcodeAction(code);
    if (!product.found || !product.name || !nameRef.current) {
      setNote(tScan("notFound", { code }));

      return;
    }

    nameRef.current.value = product.name;

    const brand = nameRef.current.form?.elements.namedItem("brand");
    if (brand instanceof HTMLInputElement && product.brand) brand.value = product.brand;

    setNote(tScan("found", { name: product.name }));
    setScanning(false);
    void check(product.name);
  }

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
        ref={nameRef}
        name="name"
        required
        maxLength={120}
        onBlur={(event) => void check(event.target.value)}
        onChange={() => setSimilar([])}
      />

      <button
        type="button"
        onClick={() => setScanning((on) => !on)}
        className="mt-2 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
      >
        {tScan("scanButton")}
      </button>

      {scanning && (
        <div className="mt-2">
          <BarcodeScanner onCode={(code) => void onScan(code)} onClose={() => setScanning(false)} />
        </div>
      )}
      {note && (
        <p role="status" className="mt-2 text-sm text-muted">
          {note}
        </p>
      )}

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
