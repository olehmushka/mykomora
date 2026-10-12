"use server";

import { lookupBarcode, type BarcodeProduct } from "@/lib/api/items";

/** Never throws: an unreachable catalogue or a bad code is just "not found". */
export async function lookupBarcodeAction(code: string): Promise<BarcodeProduct> {
  const result = await lookupBarcode(code);

  return result.ok ? result.data : { code, found: false };
}
