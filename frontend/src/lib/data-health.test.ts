import { describe, expect, it } from "vitest";
import type { AccountHealth, DataHealth } from "@/lib/api/finance";
import { issues } from "./data-health";

const now = new Date("2026-11-14T12:00:00").getTime();
const account = (over: Partial<AccountHealth>): AccountHealth =>
  ({ id: "a", name: "c", institution: "i", type: "BANK", manual: false, last_date: "2026-11-13", check_since: null, gap: null, ...over });
const ok = [{ kind: "dump", ok: true, detail: "", at: "2026-11-14T03:00:00Z" }, { kind: "restore_check", ok: true, detail: "", at: "2026-11-10T03:00:00Z" }] as DataHealth["backups"];
const health = (accounts: AccountHealth[], backups = ok): DataHealth => ({ accounts, backups, uncategorized: 0, paired: 0 });

describe("issues", () => {
  it("is zero when accounts move and backups run", () => {
    expect(issues(health([account({}), account({ type: "CREDIT", last_date: "2026-09-01" })]), now)).toBe(0);
  });
  it("counts a quiet bank account, a balance gap and a failed dump", () => {
    const failed = [{ ...ok[0], ok: false }, ok[1]] as DataHealth["backups"];
    expect(issues(health([account({ last_date: "2026-11-01" }), account({ gap: -500 })], failed), now)).toBe(3);
  });
});
