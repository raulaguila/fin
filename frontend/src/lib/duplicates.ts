import type { Transaction } from "@/lib/api/finance";
import { merchantName } from "@/lib/merchant";

/** The tag a person adds to say "these charges are separate purchases". */
export const CHECKED = "conferida";

/** Charges that look billed twice: same account, day, amount and merchant. Maps each id to its group size. */
export function repeated(items: Transaction[]): Map<string, number> {
  const groups = new Map<string, Transaction[]>();
  for (const tx of items) {
    if (tx.amount >= 0 || tx.split_of) continue;
    const key = `${tx.account_id}|${tx.date}|${tx.amount}|${merchantName(tx.description)}`;
    groups.set(key, [...(groups.get(key) ?? []), tx]);
  }
  const out = new Map<string, number>();
  for (const group of groups.values()) {
    if (group.length < 2 || group.some((tx) => tx.tags?.includes(CHECKED))) continue;
    for (const tx of group) out.set(tx.id, group.length);
  }
  return out;
}
