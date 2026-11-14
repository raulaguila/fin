import { describe, expect, it } from "vitest";
import type { Transaction } from "@/lib/api/finance";
import { CHECKED, repeated } from "./duplicates";

const tx = (id: string, over: Partial<Transaction> = {}) =>
  ({ id, account_id: "a", date: "2026-11-12", amount: -4849, description: "Paymentwall", split_of: "", tags: null, ...over }) as Transaction;

describe("repeated", () => {
  it("flags the same charge twice on one day", () => {
    const r = repeated([tx("1"), tx("2"), tx("3", { date: "2026-11-13" }), tx("4", { amount: 4849 })]);
    expect([...r]).toEqual([["1", 2], ["2", 2]]);
  });
  it("leaves a group alone once one charge is marked as checked", () => {
    expect(repeated([tx("1"), tx("2", { tags: [CHECKED] })]).size).toBe(0);
  });
});
