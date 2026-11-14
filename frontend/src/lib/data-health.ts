import type { AccountHealth, DataHealth } from "@/lib/api/finance";

const DAY = 86_400_000;
const STALE_DAYS = 7;
/** Below R$ 1 a gap is rounding at the bank, not missing data. */
const GAP_MIN = 100;

export const daysSince = (date: string, now: number) => Math.floor((now - new Date(`${date}T00:00:00`).getTime()) / DAY);

/** A bank account always moves; a card or an unused account can rest for weeks. */
export const isStale = (a: AccountHealth, now: number) => !a.manual && a.type === "BANK" && !!a.last_date && daysSince(a.last_date, now) > STALE_DAYS;

export const gapOf = (a: AccountHealth) => (a.gap !== null && Math.abs(a.gap) >= GAP_MIN ? a.gap : 0);

/** A daily step older than two days, or the weekly check older than eight, means the backup service stopped. */
export const backupOld = (kind: DataHealth["backups"][number]["kind"], at: string, now: number) =>
  now - new Date(at).getTime() > (kind === "restore_check" ? 8 : 2) * DAY;

/** What needs a look: stale or unbalanced accounts, and a daily backup or restore check that failed or stopped. */
export function issues(h: DataHealth, now: number): number {
  const accounts = h.accounts.filter((a) => isStale(a, now) || gapOf(a) !== 0).length;
  const backups = (["dump", "restore_check"] as const).filter((kind) => {
    const run = h.backups.find((b) => b.kind === kind);
    return !run || !run.ok || backupOld(kind, run.at, now);
  }).length;
  return accounts + backups;
}
