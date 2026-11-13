import { describe, expect, it } from "vitest";
import { PART, archiveParts } from "./archive";

describe("archiveParts", () => {
  it("sends settings first, then accounts and overrides in parts", () => {
    const txs = Array.from({ length: PART + 1 }, (_, seq) => ({ seq }));
    const parts = archiveParts({
      version: 1,
      categories: [{ name: "Mercado" }],
      accounts: [{ name: "Carteira", transactions: txs }, { name: "Vazia" }],
      overrides: [{ external_id: "x" }],
    });
    expect(parts).toHaveLength(5);
    expect(parts[0]).toEqual({ version: 1, categories: [{ name: "Mercado" }] });
    expect((parts[1].accounts as { transactions: unknown[] }[])[0].transactions).toHaveLength(PART);
    expect((parts[2].accounts as { transactions: unknown[] }[])[0].transactions).toHaveLength(1);
    expect(parts[3]).toEqual({ version: 1, accounts: [{ name: "Vazia", transactions: [] }] });
    expect(parts[4]).toEqual({ version: 1, overrides: [{ external_id: "x" }] });
  });

  it("refuses what is not an archive", () => {
    expect(() => archiveParts([1, 2])).toThrow();
    expect(() => archiveParts(null)).toThrow();
  });
});
