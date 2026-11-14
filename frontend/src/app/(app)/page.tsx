"use client";

import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowDownLeft, ArrowUpRight, CreditCard, ShieldCheck, Wallet, type LucideIcon } from "lucide-react";
import { CategoryDot, MonthSwitcher, Meter, NetWorthChart, NoConnections, TrendChart } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { QueryErrorState } from "@/components/query-error-state";
import { SetupChecklist } from "@/components/setup-checklist";
import { ListSkeleton } from "@/components/skeletons";
import { Card } from "@/components/ui/card";
import { finance, monthKey, type Connection, type Summary } from "@/lib/api/finance";
import { issues } from "@/lib/data-health";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import type { MessageKey } from "@/lib/i18n/messages";
import { merchantName } from "@/lib/merchant";
import { cn } from "@/lib/utils";

function Stat({ label, value, note, icon: Icon, tone }: { label: string; value: string; note?: React.ReactNode; icon: LucideIcon; tone?: string }) {
  return (
    <Card className="min-w-0 gap-1.5 p-3 sm:p-4">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-xs text-muted-foreground sm:text-sm">{label}</span>
        <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      </div>
      <span className={cn("truncate text-lg font-semibold tabular-nums sm:text-2xl", tone)}>{value}</span>
      {note && <span className="text-xs text-muted-foreground max-sm:line-clamp-2">{note}</span>}
    </Card>
  );
}

/** Consent about to expire or a broken connection, first thing on the page. */
function ConnectionNotices({ connections }: { connections: Connection[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [soon] = useState(() => Date.now() + 15 * 86_400_000);
  const notices = connections.flatMap((c) => {
    const out: string[] = [];
    if (c.status === "error" || c.status === "login_required") out.push(fill(t("fin.dash.conn_problem"), { b: c.institution }));
    if (c.consent_expires_at && Date.parse(c.consent_expires_at) < soon) out.push(fill(t("fin.dash.consent_warn"), { b: c.institution, d: f.date(c.consent_expires_at) }));
    return out;
  });
  if (notices.length === 0) return null;
  return (
    <Link href="/connections" className="flex items-center gap-3 rounded-xl border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs hover:bg-amber-500/15 sm:p-4 sm:text-sm">
      <AlertTriangle className="size-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
      <div className="min-w-0 flex-1 space-y-1">{notices.map((n) => <p key={n}>{n}</p>)}</div>
      <span className="shrink-0 font-medium underline-offset-2 max-sm:hidden">{t("fin.dash.open_connections")}</span>
    </Link>
  );
}

function Overview({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const banks = s.accounts.filter((a) => a.type === "BANK");
  const cards = s.accounts.filter((a) => a.type === "CREDIT");
  const balance = banks.reduce((n, a) => n + a.balance, 0);
  const bill = cards.reduce((n, a) => n + a.balance, 0);
  const nextDue = cards.map((a) => a.next_bill_due).filter((d): d is string => !!d).sort()[0];
  const last = cards.filter((a) => a.last_bill !== null && a.last_bill_due).sort((a, b) => (b.last_bill_due ?? "").localeCompare(a.last_bill_due ?? ""))[0];
  const billNote = !cards.length ? t("fin.dash.no_cards") : (
    <>
      {nextDue && fill(t("fin.dash.next_due"), { d: f.day(nextDue) })}
      {last && <span className="block">{fill(t("fin.dash.last_bill"), { v: f.money(last.last_bill ?? 0), d: f.day(last.last_bill_due ?? "") })}</span>}
    </>
  );
  return (
    <div className="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4">
      <Stat label={t("fin.dash.balance")} value={f.money(balance)} icon={Wallet}
        note={fill(t("fin.dash.accounts_n"), { n: banks.length, b: new Set(banks.map((a) => a.connection_id)).size })} />
      <Stat label={t("fin.dash.income")} value={f.money(s.income)} icon={ArrowDownLeft} tone={amountClass(s.income)} />
      <Stat label={t("fin.dash.expense")} value={f.money(-s.expense)} icon={ArrowUpRight}
        note={fill(t("fin.dash.left"), { v: f.money(s.income + s.expense) })} />
      <Stat label={t("fin.dash.card_bill")} value={cards.length ? f.money(bill) : "—"} icon={CreditCard}
        note={billNote} />
    </div>
  );
}

function Forecast({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const fc = s.forecast;
  if (!fc) return null;
  const perDay = fc.days_left > 0 ? Math.trunc(fc.projected / fc.days_left) : 0;
  return (
    <Card className="gap-3 p-4">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-medium">{t("fin.dash.forecast")}</h2>
        <Link href="/calendar" className="text-sm text-muted-foreground hover:underline">{t("nav.calendar")}</Link>
      </div>
      <div className="flex flex-wrap items-end gap-x-8 gap-y-3">
        <div>
          <p className="text-xs text-muted-foreground">{t("fin.dash.projected")}</p>
          <p className={cn("text-2xl font-semibold tabular-nums", fc.projected < 0 && "text-red-700 dark:text-red-400")}>{f.money(fc.projected)}</p>
          {fc.days_left > 0 && fc.projected > 0 && <p className="text-xs text-muted-foreground">{fill(t("fin.dash.per_day"), { v: f.money(perDay), n: fc.days_left })}</p>}
        </div>
        <dl className="grid grid-cols-[auto_auto] gap-x-4 gap-y-1 text-sm">
          <dt className="text-muted-foreground">{t("fin.dash.pending_bills")}</dt>
          <dd className="text-right font-mono tabular-nums">{f.money(-fc.pending_bills)}</dd>
          <dt className="text-muted-foreground">{t("fin.dash.expected_income")}</dt>
          <dd className="text-right font-mono tabular-nums">{f.money(fc.expected_income)}</dd>
        </dl>
      </div>
      <p className="text-xs text-muted-foreground">{t("fin.dash.forecast_note")}</p>
    </Card>
  );
}

function Trend() {
  const { t } = useI18n();
  const trend = useQuery({ queryKey: ["finance", "trend"], queryFn: () => finance.trend(12) });
  if (!trend.data?.items.some((m) => m.income || m.expense)) return null;
  return (
    <Card className="gap-3 p-4">
      <h2 className="text-sm font-medium">{t("fin.dash.trend")}</h2>
      <TrendChart items={trend.data.items} />
    </Card>
  );
}

function ByCategory({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const rows = s.categories.filter((c) => c.kind === "expense" && c.total < 0).sort((a, b) => a.total - b.total);
  const top = rows[0] ? -rows[0].total : 0;
  // Uncategorized money growing means rows to sort, not spending: it never counts as rising.
  const rising = rows.find((c) => c.category_id && c.previous < 0 && c.total < c.previous * 1.2 && c.total - c.previous < -5000);
  return (
    <Card className="gap-3 p-4">
      <h2 className="text-sm font-medium">{t("fin.dash.by_category")}</h2>
      {rising && (
        <p className="text-sm text-amber-800 dark:text-amber-300">
          {fill(t("fin.dash.above_prev"), { c: rising.name, p: Math.round((rising.total / rising.previous - 1) * 100) })}
        </p>
      )}
      {rows.length === 0 && <p className="text-sm text-muted-foreground">{t("fin.dash.nothing")}</p>}
      <ul className="space-y-3">
        {rows.map((c) => (
          <li key={c.category_id || "none"} className="space-y-1">
            <div className="flex items-baseline justify-between gap-3 text-sm">
              <Link href={`/transactions?category=${c.category_id || "none"}&month=${s.month}`} className="truncate hover:underline">{c.name || t("fin.uncategorized")}</Link>
              <span className="shrink-0 font-mono tabular-nums">{f.money(-c.total)}</span>
            </div>
            <Meter value={-c.total} max={c.limit > 0 ? c.limit : top} neutral={c.limit <= 0} />
            {c.previous < 0 && <p className="text-xs text-muted-foreground">{fill(t("fin.dash.vs_prev"), { p: f.money(-c.previous) })}</p>}
          </li>
        ))}
      </ul>
    </Card>
  );
}

function Recent({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  return (
    <Card className="gap-3 p-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-medium">{t("fin.dash.recent")}</h2>
        <Link href={`/transactions?month=${s.month}`} className="text-sm text-muted-foreground hover:underline">{t("fin.dash.see_all")}</Link>
      </div>
      {s.recent.length === 0 && <p className="text-sm text-muted-foreground">{t("fin.dash.nothing")}</p>}
      <ul className="divide-y">
        {s.recent.map((tx) => (
          <li key={tx.id} className="flex items-center gap-3 py-2 text-sm">
            <CategoryDot name={tx.category_name} />
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium" title={tx.description}>{merchantName(tx.description)}</p>
              <p className="truncate text-xs text-muted-foreground">{f.day(tx.date)} · {tx.category_name || t("fin.uncategorized")} · {tx.account_name}</p>
            </div>
            <span className={cn("shrink-0 font-mono tabular-nums", amountClass(tx.amount))}>{f.money(tx.amount)}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

const INVESTMENT_TYPES = ["FIXED_INCOME", "SECURITY", "MUTUAL_FUND", "EQUITY", "COE", "ETF", "OTHER"];

/** Net worth: bank balances and investments, less what the cards owe. */
function Wealth({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const history = useQuery({ queryKey: ["finance", "networth"], queryFn: () => finance.netWorth(12) });
  const banks = s.accounts.reduce((n, a) => n + (a.type === "BANK" ? a.balance : 0), 0);
  const cards = s.accounts.reduce((n, a) => n + (a.type === "CREDIT" ? a.balance : 0), 0);
  const invested = s.investments.reduce((n, i) => n + i.balance, 0);
  return (
    <Card className="gap-3 p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-sm font-medium">{t("fin.dash.net_worth")}</h2>
        <span className="text-2xl font-semibold tabular-nums">{f.money(banks + invested - cards)}</span>
      </div>
      <p className="text-xs text-muted-foreground">
        {fill(t("fin.dash.net_worth_parts"), { b: f.money(banks), i: f.money(invested), c: f.money(cards) })}
      </p>
      {history.data && <NetWorthChart items={history.data.items} />}
      {s.investments.length > 0 && <ul className="divide-y">
        {s.investments.map((i) => (
          <li key={i.id} className="flex items-center justify-between gap-3 py-2 text-sm">
            <div className="min-w-0">
              <p className="truncate">{i.name}</p>
              <p className="truncate text-xs text-muted-foreground">{s.institutions[i.connection_id]}{i.type && ` · ${INVESTMENT_TYPES.includes(i.type) ? t(`fin.inv.${i.type}` as MessageKey) : i.type}`}</p>
            </div>
            <span className="shrink-0 font-mono tabular-nums">{f.money(i.balance)}</span>
          </li>
        ))}
      </ul>}
    </Card>
  );
}

/** One glance at /data: green when accounts move and backups run, amber with a count otherwise. */
function HealthBadge() {
  const { t } = useI18n();
  const health = useQuery({ queryKey: ["finance", "health"], queryFn: finance.health });
  const [now] = useState(Date.now);
  if (!health.data) return null;
  const n = issues(health.data, now);
  return (
    <Link href="/data" className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium hover:opacity-80",
      n ? "bg-amber-500/15 text-amber-800 dark:text-amber-300" : "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300")}>
      {n ? <AlertTriangle className="size-3.5" aria-hidden="true" /> : <ShieldCheck className="size-3.5" aria-hidden="true" />}
      {n ? fill(t("fin.data.badge_issues"), { n }) : t("fin.data.badge_ok")}
    </Link>
  );
}

export default function DashboardPage() {
  const { t } = useI18n();
  const [params, set] = useListParams({ month: monthKey() });
  const summary = useQuery({ queryKey: ["finance", "summary", params.month], queryFn: () => finance.summary(params.month) });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const s = summary.data;
  return (
    <>
      <PageHeader title={t("nav.dashboard")} description={t("fin.dash.subtitle")}
        action={<div className="flex flex-wrap items-center gap-2"><HealthBadge /><MonthSwitcher month={params.month} onChange={(m) => set({ month: m })} /></div>} />
      <PageBody>
        {connections.data && <ConnectionNotices connections={connections.data.items} />}
        <SetupChecklist />
        {summary.isPending ? (
          <ListSkeleton />
        ) : summary.isError ? (
          <QueryErrorState error={summary.error} onRetry={() => void summary.refetch()} />
        ) : connections.data?.items.length === 0 ? (
          <NoConnections />
        ) : (
          s && (
            <>
              <Overview s={s} />
              <div className="grid gap-4 lg:grid-cols-2">
                <Forecast s={s} />
                <Trend />
              </div>
              <div className="grid gap-4 lg:grid-cols-2">
                <ByCategory s={s} />
                <Recent s={s} />
              </div>
              <Wealth s={s} />
            </>
          )
        )}
      </PageBody>
    </>
  );
}
