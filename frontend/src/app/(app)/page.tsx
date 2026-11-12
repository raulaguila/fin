"use client";

import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowDownLeft, ArrowUpRight, CreditCard, Wallet, type LucideIcon } from "lucide-react";
import { MonthSwitcher, Meter, NoConnections } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, monthKey, type Connection, type Summary } from "@/lib/api/finance";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

function Stat({ label, value, note, icon: Icon, tone }: { label: string; value: string; note?: React.ReactNode; icon: LucideIcon; tone?: string }) {
  return (
    <Card className="gap-1.5 p-4">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">{label}</span>
        <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      </div>
      <span className={cn("text-2xl font-semibold tabular-nums", tone)}>{value}</span>
      {note && <span className="text-xs text-muted-foreground">{note}</span>}
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
    <Card className="flex-row flex-wrap items-center gap-3 border-amber-500/40 bg-amber-500/10 p-4">
      <AlertTriangle className="size-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
      <div className="min-w-0 flex-1 space-y-1 text-sm">{notices.map((n) => <p key={n}>{n}</p>)}</div>
      <Button size="sm" variant="outline" nativeButton={false} render={<Link href="/connections" />}>{t("fin.dash.open_connections")}</Button>
    </Card>
  );
}

function Overview({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const banks = s.accounts.filter((a) => a.type === "BANK");
  const cards = s.accounts.filter((a) => a.type === "CREDIT");
  const balance = banks.reduce((n, a) => n + a.balance, 0);
  const bill = cards.reduce((n, a) => n + a.balance, 0);
  const nextDue = cards.map((a) => a.bill_due_date).filter((d): d is string => !!d).sort()[0];
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 xl:grid-cols-4">
      <Stat label={t("fin.dash.balance")} value={f.money(balance)} icon={Wallet}
        note={fill(t("fin.dash.accounts_n"), { n: banks.length, b: new Set(banks.map((a) => a.connection_id)).size })} />
      <Stat label={t("fin.dash.income")} value={f.money(s.income)} icon={ArrowDownLeft} tone={amountClass(s.income)} />
      <Stat label={t("fin.dash.expense")} value={f.money(-s.expense)} icon={ArrowUpRight}
        note={fill(t("fin.dash.left"), { v: f.money(s.income + s.expense) })} />
      <Stat label={t("fin.dash.card_bill")} value={cards.length ? f.money(bill) : "—"} icon={CreditCard}
        note={cards.length ? nextDue && fill(t("fin.dash.due"), { d: f.day(nextDue) }) : t("fin.dash.no_cards")} />
    </div>
  );
}

function ByCategory({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const rows = s.categories.filter((c) => c.kind === "expense" && c.total < 0).sort((a, b) => a.total - b.total);
  const top = rows[0] ? -rows[0].total : 0;
  const rising = rows.find((c) => c.previous < 0 && c.total < c.previous * 1.2 && c.total - c.previous < -5000);
  return (
    <Card className="gap-3 p-4">
      <h2 className="text-sm font-medium">{t("fin.dash.by_category")}</h2>
      {rising && (
        <p className="text-sm text-amber-800 dark:text-amber-300">
          {fill(t("fin.dash.above_prev"), { c: rising.name || t("fin.uncategorized"), p: Math.round((rising.total / rising.previous - 1) * 100) })}
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
          <li key={tx.id} className="flex items-center justify-between gap-3 py-2 text-sm">
            <div className="min-w-0">
              <p className="truncate font-medium">{tx.description}</p>
              <p className="truncate text-xs text-muted-foreground">{f.day(tx.date)} · {tx.category_name || t("fin.uncategorized")} · {tx.account_name}</p>
            </div>
            <span className={cn("shrink-0 font-mono tabular-nums", amountClass(tx.amount))}>{f.money(tx.amount)}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function Commitments({ s }: { s: Summary }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  if (s.recurring.length === 0 && s.installments.length === 0) return null;
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {s.recurring.length > 0 && (
        <Card className="gap-3 p-4">
          <h2 className="text-sm font-medium">{t("fin.dash.recurring")}</h2>
          <ul className="divide-y">
            {s.recurring.map((r) => (
              <li key={r.name} className="flex items-center justify-between gap-3 py-2 text-sm">
                <div className="min-w-0">
                  <p className="truncate">{r.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {fill(t("fin.dash.months_seen"), { n: r.months })}
                    {r.changed && <span className="text-amber-800 dark:text-amber-300"> · {fill(t("fin.dash.recurring_changed"), { v: f.money(-r.previous) })}</span>}
                  </p>
                </div>
                <span className="shrink-0 font-mono tabular-nums">{f.money(-r.amount)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
      {s.installments.length > 0 && (
        <Card className="gap-3 p-4">
          <h2 className="text-sm font-medium">{t("fin.dash.installments")}</h2>
          <ul className="divide-y">
            {s.installments.map((i) => (
              <li key={i.description} className="flex items-center justify-between gap-3 py-2 text-sm">
                <div className="min-w-0">
                  <p className="truncate">{i.description}</p>
                  <p className="text-xs text-muted-foreground">{fill(t("fin.dash.remaining"), { n: i.remaining })}</p>
                </div>
                <span className="shrink-0 font-mono tabular-nums">{f.money(-i.amount)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
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
      <PageHeader title={t("nav.dashboard")} description={t("fin.dash.subtitle")} action={<MonthSwitcher month={params.month} onChange={(m) => set({ month: m })} />} />
      <PageBody>
        {connections.data && <ConnectionNotices connections={connections.data.items} />}
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
                <ByCategory s={s} />
                <Recent s={s} />
              </div>
              <Commitments s={s} />
            </>
          )
        )}
      </PageBody>
    </>
  );
}
