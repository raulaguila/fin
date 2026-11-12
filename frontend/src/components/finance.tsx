"use client";

import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Landmark } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { finance, monthKey, shiftMonth, type Account, type MonthTotal } from "@/lib/api/finance";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { financePt as MESSAGES_PT } from "@/lib/i18n/finance-messages";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export const useFinanceStatus = () => useQuery({ queryKey: ["finance", "status"], queryFn: finance.status, staleTime: 5 * 60_000 });

/** Previous / next month around the month label; never past the current month. */
export function MonthSwitcher({ month, onChange }: { month: string; onChange: (m: string) => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  return (
    <div className="flex items-center gap-1">
      <Button variant="ghost" size="icon" aria-label={t("fin.prev_month")} onClick={() => onChange(shiftMonth(month, -1))}>
        <ChevronLeft className="size-4" />
      </Button>
      <span className="min-w-36 text-center text-sm font-medium tabular-nums">{f.month(month)}</span>
      <Button variant="ghost" size="icon" aria-label={t("fin.next_month")} disabled={month >= monthKey()} onClick={() => onChange(shiftMonth(month, 1))}>
        <ChevronRight className="size-4" />
      </Button>
    </div>
  );
}

/** Shown where data needs a bank and the user has none. */
export function NoConnections() {
  const { t } = useI18n();
  return (
    <EmptyState
      icon={Landmark}
      title={t("fin.no_connections_title")}
      description={t("fin.no_connections_body")}
      action={<Button nativeButton={false} render={<Link href="/connections" />}>{t("fin.connect_bank")}</Button>}
    />
  );
}

/** Spent share of a limit: amber from 80%, red from 100%. Neutral when max is only a scale, not a limit. */
export function Meter({ value, max, neutral, className }: { value: number; max: number; neutral?: boolean; className?: string }) {
  const pct = max > 0 ? Math.min(100, Math.round((value / max) * 100)) : 0;
  const ratio = max > 0 ? value / max : 0;
  return (
    <div className={cn("h-2 w-full overflow-hidden rounded-full bg-muted", className)} role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
      <div
        className={cn("h-full rounded-full transition-[width]", neutral ? "bg-primary/70" : ratio >= 1 ? "bg-red-500" : ratio >= 0.8 ? "bg-amber-500" : "bg-emerald-500")}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

/** "itau · Poupança · final 1234": tells apart two accounts of one bank. */
export function useAccountLabel() {
  const { t } = useI18n();
  return (a: Account) => {
    const parts = [a.name];
    const kind = `fin.acc.${a.subtype}`;
    if (a.subtype && kind in MESSAGES_PT) parts.push(t(kind as MessageKey));
    const digits = a.number.replace(/\D/g, "");
    if (digits.length >= 4) parts.push(fill(t("fin.acc.final"), { n: digits.slice(-4) }));
    return parts.join(" · ");
  };
}

/**
 * Income and spending by month as paired bars on one zero baseline. Hover or
 * focus a month for its numbers; a visually hidden table carries the same data.
 * Colors: categorical slots 1 and 2, validated for both themes and color blindness.
 */
export function TrendChart({ items }: { items: MonthTotal[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [active, setActive] = useState<number | null>(null);
  const top = Math.max(1, ...items.map((m) => Math.max(m.income, -m.expense)));
  const shown = active ?? items.length - 1;
  const cur = items[shown];
  const short = (m: string) => f.month(m).slice(0, 3);
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-sm">
        <span className="font-medium">{cur && f.month(cur.month)}</span>
        {cur && (
          <span className="flex flex-wrap gap-x-4 gap-y-1 tabular-nums text-muted-foreground">
            <span><Swatch tone="in" />{t("fin.dash.income")} <span className="text-foreground">{f.money(cur.income)}</span></span>
            <span><Swatch tone="out" />{t("fin.dash.expense")} <span className="text-foreground">{f.money(-cur.expense)}</span></span>
            <span>{fill(t("fin.dash.trend_left"), { v: f.money(cur.income + cur.expense) })}</span>
          </span>
        )}
      </div>
      <div className="flex h-40 items-end gap-1 border-b border-border" aria-hidden="true" onMouseLeave={() => setActive(null)}>
        {items.map((m, i) => (
          <div key={m.month} className={cn("flex h-full min-w-0 flex-1 cursor-default items-end justify-center gap-0.5 rounded-sm", i === shown && "bg-muted/60")}
            onMouseEnter={() => setActive(i)}>
            <div className="w-full max-w-3 rounded-t bg-[#2a78d6] dark:bg-[#3987e5]" style={{ height: `${(m.income / top) * 100}%` }} />
            <div className="w-full max-w-3 rounded-t bg-[#eb6834] dark:bg-[#d95926]" style={{ height: `${(-m.expense / top) * 100}%` }} />
          </div>
        ))}
      </div>
      <div className="flex gap-1 text-[10px] text-muted-foreground" aria-hidden="true">
        {items.map((m, i) => (
          <button key={m.month} type="button" tabIndex={-1} className={cn("min-w-0 flex-1 truncate text-center", i === shown && "font-medium text-foreground")}
            onClick={() => setActive(i)}>{short(m.month)}</button>
        ))}
      </div>
      <table className="sr-only">
        <caption>{t("fin.dash.trend_table")}</caption>
        <thead><tr><th>{t("fin.dash.trend")}</th><th>{t("fin.dash.income")}</th><th>{t("fin.dash.expense")}</th></tr></thead>
        <tbody>
          {items.map((m) => <tr key={m.month}><td>{f.month(m.month)}</td><td>{f.money(m.income)}</td><td>{f.money(-m.expense)}</td></tr>)}
        </tbody>
      </table>
    </div>
  );
}

function Swatch({ tone }: { tone: "in" | "out" }) {
  return <span className={cn("mr-1.5 inline-block size-2.5 rounded-sm align-middle", tone === "in" ? "bg-[#2a78d6] dark:bg-[#3987e5]" : "bg-[#eb6834] dark:bg-[#d95926]")} />;
}
