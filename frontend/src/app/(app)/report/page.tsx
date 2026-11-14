"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ChevronLeft, ChevronRight, FileText, Printer } from "lucide-react";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { CategoryDot } from "@/components/finance";
import { MoneyFlow } from "@/components/money-flow";
import { QueryErrorState } from "@/components/query-error-state";
import { MonthBars, ReportDetail } from "@/components/report-detail";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { finance, monthKey, type CategoryTotal, type YearReport } from "@/lib/api/finance";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { TRANSFER_LIKE } from "@/lib/merchant";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

/** Expenses the income tax return deducts (health and education), by the default category names. */
const DEDUCTIBLE = ["saúde", "educação"];
const isDeductible = (c: CategoryTotal) => DEDUCTIBLE.includes(c.name.toLowerCase());
/** Spending categories that often hold money moved to the user's own accounts. */
const TRANSFER_SHARE = 0.2;
const IN = "#1e9e73", OUT = "#eb6834";
const YEAR = "year";

const key = (c: CategoryTotal) => `${c.category_id}${c.kind}`;
const byKind = (cats: CategoryTotal[], kind: "income" | "expense") =>
  cats.filter((c) => c.kind === kind && c.total !== 0).sort((a, b) => (kind === "income" ? b.total - a.total : a.total - b.total));
const pct = (n: number) => `${n > 0 ? "+" : ""}${Math.round(n * 100)}%`;
/** Change against last year, or nothing when last year had no such money. */
const change = (now: number, before: number | undefined) => (before ? (Math.abs(now) - Math.abs(before)) / Math.abs(before) : undefined);
const delta = (now: number, before: number | undefined) => {
  const d = change(now, before);
  return d === undefined ? undefined : pct(d);
};
const savings = (r: Pick<YearReport, "income" | "expense">) => (r.income > 0 ? (r.income + r.expense) / r.income : undefined);

type Picked = { title: string; cats: CategoryTotal[]; month?: string };

function Kpi({ label, value, note, help }: { label: string; value: string; note?: string; help?: string }) {
  return (
    <Card className="gap-1 p-4">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-2xl font-semibold tabular-nums">{value}</span>
      {note && <span className="text-xs text-muted-foreground tabular-nums">{note}</span>}
      {help && <span className="text-xs text-muted-foreground">{help}</span>}
    </Card>
  );
}

function CategoryTable({ title, rows, total, prev, onOpen }: {
  title: string; rows: CategoryTotal[]; total: number; prev?: Map<string, number>; onOpen: (c: CategoryTotal) => void;
}) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const all = Math.abs(total) || 1;
  return (
    <Card className="gap-0 overflow-hidden p-0">
      <h2 className="px-4 pt-4 pb-2 text-sm font-medium">{title}</h2>
      <Table data-compact>
        <TableHeader>
          <TableRow>
            <TableHead>{t("fin.tx.category")}</TableHead>
            <TableHead className="w-28 max-sm:hidden">{t("fin.report.share")}</TableHead>
            {prev && <TableHead className="w-24 text-right max-sm:hidden">{t("fin.report.change")}</TableHead>}
            <TableHead className="w-36 text-right">{t("fin.tx.amount")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((c) => {
            const share = Math.abs(c.total) / all;
            const d = prev && change(c.total, prev.get(key(c)));
            return (
              <TableRow key={key(c)} className="cursor-pointer" onClick={() => onOpen(c)}>
                <TableCell>
                  <button type="button" className="flex min-w-0 items-center gap-2 text-left hover:underline" onClick={(e) => { e.stopPropagation(); onOpen(c); }}
                    aria-label={fill(t("fin.report.open"), { c: c.name || t("fin.uncategorized") })}>
                    <CategoryDot name={c.name} className="size-6 [&_svg]:size-3.5" />
                    <span className="truncate">{c.name || t("fin.uncategorized")}</span>
                    {isDeductible(c) && <span className="rounded-full bg-sky-500/15 px-2 py-0.5 text-xs text-sky-800 dark:text-sky-300">{t("fin.report.deductible")}</span>}
                  </button>
                </TableCell>
                <TableCell className="max-sm:hidden">
                  <div className="flex items-center gap-2">
                    <div className="h-1.5 flex-1 rounded-full bg-muted"><div className="h-1.5 rounded-full bg-primary/70" style={{ width: `${share * 100}%` }} /></div>
                    <span className="w-9 text-right text-xs text-muted-foreground tabular-nums">{Math.round(share * 100)}%</span>
                  </div>
                </TableCell>
                {prev && <TableCell className="text-right text-xs text-muted-foreground tabular-nums max-sm:hidden">{d === undefined ? "—" : pct(d)}</TableCell>}
                <TableCell className={cn("text-right font-mono tabular-nums", amountClass(c.total))}>{f.money(Math.abs(c.total))}</TableCell>
              </TableRow>
            );
          })}
          <TableRow>
            <TableCell className="font-medium">{t("fin.report.total")}</TableCell>
            <TableCell className="max-sm:hidden" />
            {prev && <TableCell className="max-sm:hidden" />}
            <TableCell className="text-right font-mono font-medium tabular-nums">{f.money(Math.abs(total))}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>
  );
}

export default function ReportPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const thisYear = new Date().getFullYear();
  const [params, set] = useListParams({ year: thisYear, period: YEAR });
  const year = Number(params.year) || thisYear;
  const period = params.period === YEAR || !params.period.startsWith(`${year}-`) ? YEAR : params.period;
  const [picked, setPicked] = useState<Picked | null>(null);
  const report = useQuery({ queryKey: ["finance", "year", year], queryFn: () => finance.year(year) });
  const before = useQuery({ queryKey: ["finance", "year", year - 1], queryFn: () => finance.year(year - 1), enabled: year > 2000 });
  const month = useQuery({ queryKey: ["finance", "summary", period], queryFn: () => finance.summary(period), enabled: period !== YEAR });
  const r = report.data;
  const income = byKind(r?.categories ?? [], "income");
  const expense = byKind(r?.categories ?? [], "expense");
  const deductible = expense.filter(isDeductible).reduce((n, c) => n + c.total, 0);
  // Only months with money in both years compare: a year the bank history starts midway would read as +2000%.
  // Only closed months: the current one is half over and future ones hold just card installments.
  const filled = (y?: YearReport) => new Set((y?.months ?? []).filter((m) => (m.income || m.expense) && m.month < monthKey()).map((m) => m.month.slice(5)));
  const mine = filled(r), theirs = filled(before.data);
  const common = new Set([...mine].filter((m) => theirs.has(m)));
  const whole = common.size === mine.size && common.size === theirs.size;
  const within = (y: YearReport) => y.months.filter((m) => common.has(m.month.slice(5))).reduce((n, m) => ({ income: n.income + m.income, expense: n.expense + m.expense }), { income: 0, expense: 0 });
  const p = before.data && common.size > 0 ? within(before.data) : undefined;
  const now = r && p ? within(r) : r;
  // Category totals are for the whole year, so they compare only when both years cover the same months.
  const prev = p && whole && before.data ? new Map(before.data.categories.map((c) => [key(c), c.total])) : undefined;
  const flowCats = period === YEAR ? r?.categories : month.data?.categories;
  const rate = r && savings(r);
  const rateNow = now && savings(now);
  const rateBefore = p && savings(p);
  const vs = (d: string | undefined) => (d === undefined ? undefined : fill(t(whole ? "fin.report.vs_prev" : "fin.report.vs_prev_months"), { d, y: year - 1, n: common.size }));
  const transferLike = r && expense.find((c) => TRANSFER_LIKE.test(c.name) && Math.abs(c.total) / Math.abs(r.expense || 1) > TRANSFER_SHARE);

  const periods: Record<string, string> = { [YEAR]: t("fin.report.whole_year") };
  for (const m of r?.months ?? []) periods[m.month] = f.month(m.month);
  const pickMonth = (m: string) => {
    set({ period: m });
    document.getElementById("flow")?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <>
      <PageHeader title={t("nav.report")} description={t("fin.report.subtitle")}
        action={
          <div className="flex items-center gap-1 print:hidden">
            <Button variant="ghost" size="icon" aria-label={t("fin.report.prev")} onClick={() => set({ year: year - 1, period: YEAR })}><ChevronLeft className="size-4" /></Button>
            <span className="min-w-14 text-center text-sm font-medium tabular-nums">{year}</span>
            <Button variant="ghost" size="icon" aria-label={t("fin.report.next")} disabled={year >= thisYear} onClick={() => set({ year: year + 1, period: YEAR })}><ChevronRight className="size-4" /></Button>
            <Button variant="outline" size="sm" onClick={() => window.print()}><Printer className="size-4" />{t("fin.report.print")}</Button>
          </div>
        } />
      <PageBody>
        {report.isPending ? (
          <ListSkeleton />
        ) : report.isError ? (
          <QueryErrorState error={report.error} onRetry={() => void report.refetch()} />
        ) : !r || (r.income === 0 && r.expense === 0) ? (
          <EmptyState icon={FileText} title={fill(t("fin.report.empty"), { y: year })} />
        ) : (
          <>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <Kpi label={t("fin.report.income")} value={f.money(r.income)} note={vs(delta(now!.income, p?.income))} />
              <Kpi label={t("fin.report.expense")} value={f.money(-r.expense)} note={vs(delta(now!.expense, p?.expense))} />
              <Kpi label={t("fin.report.savings")} value={rate === undefined ? "—" : `${Math.round(rate * 100)}%`}
                note={rateNow !== undefined && rateBefore !== undefined ? vs(`${rateNow >= rateBefore ? "+" : ""}${Math.round((rateNow - rateBefore) * 100)} p.p.`) : t("fin.report.savings_help")} />
              <Kpi label={t("fin.report.deductible_total")} value={f.money(-deductible)} help={t("fin.report.deductible_help")} />
            </div>

            {transferLike && (
              <Card className="flex-row items-start gap-3 border-amber-500/40 bg-amber-500/5 p-4 text-sm print:hidden">
                <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600" aria-hidden="true" />
                <div className="grid gap-1">
                  <p>{fill(t("fin.report.transfer_warn"), { c: transferLike.name, p: `${Math.round((Math.abs(transferLike.total) / Math.abs(r.expense)) * 100)}%` })}</p>
                  <Link href="/categories" className="font-medium text-primary hover:underline">{t("fin.report.transfer_fix")}</Link>
                </div>
              </Card>
            )}

            <Card id="flow" className="scroll-mt-4 gap-3 p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-sm font-medium">{t("fin.flow.title")}</h2>
                <Select value={period} items={periods} onValueChange={(v) => set({ period: String(v ?? YEAR) })}>
                  <SelectTrigger size="sm" className="w-44 print:hidden" aria-label={t("fin.report.period")}><SelectValue /></SelectTrigger>
                  <SelectContent>{Object.entries(periods).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
                </Select>
              </div>
              {period !== YEAR && month.isPending ? (
                <ListSkeleton />
              ) : (
                <MoneyFlow income={byKind(flowCats ?? [], "income")} expense={byKind(flowCats ?? [], "expense")}
                  onSelect={(title, cats) => setPicked({ title, cats, month: period === YEAR ? undefined : period })} />
              )}
            </Card>

            <Card className="gap-3 p-4">
              <h2 className="text-sm font-medium">{t("fin.report.chart")}</h2>
              <MonthBars year={year} onPick={pickMonth} series={[
                { label: t("fin.report.income"), color: IN, values: Object.fromEntries(r.months.map((m) => [m.month, m.income])) },
                { label: t("fin.report.expense"), color: OUT, values: Object.fromEntries(r.months.map((m) => [m.month, -m.expense])) },
              ]} />
              <details className="group">
                <summary className="cursor-pointer text-sm text-muted-foreground hover:text-foreground print:hidden">{t("fin.report.table")}</summary>
                <Table data-compact className="mt-2">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("fin.report.month")}</TableHead>
                      <TableHead className="text-right">{t("fin.report.income")}</TableHead>
                      <TableHead className="text-right">{t("fin.report.expense")}</TableHead>
                      <TableHead className="text-right">{t("fin.report.balance")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {r.months.map((m) => (
                      <TableRow key={m.month}>
                        <TableCell><Link className="hover:underline" href={`/?month=${m.month}`}>{f.month(m.month)}</Link></TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{f.money(m.income)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{f.money(-m.expense)}</TableCell>
                        <TableCell className={cn("text-right font-mono tabular-nums", m.income + m.expense < 0 && "text-red-700 dark:text-red-400")}>{f.money(m.income + m.expense)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </details>
            </Card>

            <div className="grid gap-4 xl:grid-cols-2">
              <CategoryTable title={t("fin.report.income")} rows={income} total={r.income} prev={prev} onOpen={(c) => setPicked({ title: c.name, cats: [c] })} />
              <CategoryTable title={t("fin.report.expense")} rows={expense} total={r.expense} prev={prev} onOpen={(c) => setPicked({ title: c.name, cats: [c] })} />
            </div>
            <p className="text-xs text-muted-foreground">{t("fin.report.note")}</p>
          </>
        )}
      </PageBody>
      {picked && <ReportDetail key={picked.title + (picked.month ?? "")} year={year} month={picked.month} title={picked.title} cats={picked.cats} onClose={() => setPicked(null)} />}
    </>
  );
}
