"use client";

import Link from "next/link";
import { createElement, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Landmark } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { finance, monthKey, shiftMonth, type Account, type MonthTotal, type NetWorthPoint } from "@/lib/api/finance";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { financePt as MESSAGES_PT } from "@/lib/i18n/finance-messages";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { categoryColor, categoryIcon } from "@/lib/merchant";
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

/** The category's icon on its color; the same category looks the same everywhere. */
export function CategoryDot({ name, className }: { name: string; className?: string }) {
  const color = categoryColor(name);
  return (
    <span aria-hidden="true" className={cn("flex size-8 shrink-0 items-center justify-center rounded-full", className)}
      style={{ backgroundColor: `color-mix(in oklch, ${color} 16%, transparent)`, color }}>
      {createElement(categoryIcon(name), { className: "size-4" })}
    </span>
  );
}

/** Net worth over time: one line, the value under the pointer above it. */
export function NetWorthChart({ items }: { items: NetWorthPoint[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [active, setActive] = useState<number | null>(null);
  if (items.length < 2) return <p className="text-xs text-muted-foreground">{t("fin.dash.net_worth_start")}</p>;
  const W = 600, H = 120, PAD = 6;
  const values = items.map((p) => p.total);
  const lo = Math.min(...values), hi = Math.max(...values);
  const span = hi - lo || 1;
  const x = (i: number) => (i / (items.length - 1)) * W;
  const y = (v: number) => PAD + (1 - (v - lo) / span) * (H - 2 * PAD);
  const path = items.map((p, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(p.total).toFixed(1)}`).join("");
  const shown = active ?? items.length - 1;
  const cur = items[shown];
  const first = items[0];
  const change = cur.total - first.total;
  const pick = (e: React.PointerEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    setActive(Math.round(((e.clientX - r.left) / r.width) * (items.length - 1)));
  };
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-sm tabular-nums">
        <span className="text-muted-foreground">{f.date(cur.day)}</span>
        <span>
          <span className="font-medium">{f.money(cur.total)}</span>{" "}
          <span className={cn("text-xs", amountClass(change))}>{change >= 0 ? "+" : ""}{f.money(change)} {fill(t("fin.dash.net_worth_since"), { d: f.date(first.day) })}</span>
        </span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="h-28 w-full touch-none overflow-visible" aria-hidden="true"
        onPointerMove={pick} onPointerDown={pick} onPointerLeave={() => setActive(null)}>
        <line x1="0" x2={W} y1={H - 0.5} y2={H - 0.5} className="stroke-border" strokeWidth="1" vectorEffect="non-scaling-stroke" />
        <path d={path} fill="none" className="stroke-[#2a78d6] dark:stroke-[#3987e5]" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
        {active !== null && <line x1={x(shown)} x2={x(shown)} y1="0" y2={H} className="stroke-muted-foreground/50" strokeWidth="1" vectorEffect="non-scaling-stroke" />}
      </svg>
      <table className="sr-only">
        <caption>{t("fin.dash.net_worth")}</caption>
        <tbody>{items.map((p) => <tr key={p.day}><td>{f.date(p.day)}</td><td>{f.money(p.total)}</td></tr>)}</tbody>
      </table>
    </div>
  );
}

function Swatch({ tone }: { tone: "in" | "out" }) {
  return <span className={cn("mr-1.5 inline-block size-2.5 rounded-sm align-middle", tone === "in" ? "bg-[#2a78d6] dark:bg-[#3987e5]" : "bg-[#eb6834] dark:bg-[#d95926]")} />;
}
