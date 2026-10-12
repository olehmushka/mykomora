import { describe, expect, it } from "vitest";

import { shoppingListText } from "@/lib/shopping-text";

describe("shoppingListText", () => {
  it("writes one line per item with the missing amount", () => {
    expect(
      shoppingListText([
        { name: "Папір", quantity: 1, minQuantity: 4, unit: "рул." },
        { name: "Мило", quantity: 0, minQuantity: 2 },
      ]),
    ).toBe("- Папір — 3 рул.\n- Мило — 2");
  });

  it("prefers the server's shortfall and trims float noise", () => {
    expect(
      shoppingListText([{ name: "Сироп", quantity: 0.1, minQuantity: 0.4, shortfall: 0.3000001 }]),
    ).toBe("- Сироп — 0.3");
  });

  it("is empty for an empty list", () => {
    expect(shoppingListText([])).toBe("");
  });
});
