export type ShoppingEntry = {
  name: string;
  quantity: number;
  minQuantity: number;
  unit?: string;
  shortfall?: number;
};

/** Snooze presets the Attention panel offers, in days. */
export const SNOOZE_PRESETS = [
  { key: "day", days: 1 },
  { key: "week", days: 7 },
  { key: "month", days: 30 },
] as const;

function amount(value: number, unit?: string): string {
  // Quantities arrive as float32 over the wire; trim representation noise
  // (0.30000001) without hiding a genuine decimal.
  const rounded = Math.round(value * 1000) / 1000;

  return unit ? `${rounded} ${unit}` : String(rounded);
}

/**
 * The shopping list as plain text, one line per item, ready to paste into a
 * messenger: "- Paper towels: 3 more (have 1 of 4 rolls)" is too chatty for a
 * shop, so it is just "- Paper towels — 3 rolls".
 */
export function shoppingListText(entries: ShoppingEntry[]): string {
  return entries
    .map((e) => {
      const need = e.shortfall ?? e.minQuantity - e.quantity;

      return `- ${e.name} — ${amount(need, e.unit)}`;
    })
    .join("\n");
}
