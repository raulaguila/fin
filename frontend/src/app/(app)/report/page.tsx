"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, FileText, Printer } from "lucide-react";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { MoneyFlow } from "@/components/money-flow";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { finance, type CategoryTotal } from "@/lib/api/finance";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

/** Expenses the income tax return deducts (health and education), by the default category names. */
const DEDUCTIBLE = ["saúde", "educação"];
const isDeductible = (c: CategoryTotal) => DEDUCTIBLE.includes(c.name.toLowerCase());

function CategoryTable({ title, rows, total, year }: { title: string; rows: CategoryTotal[]; total: number; year: number }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  return (
    <Card className="gap-0 overflow-hidden p-0">
      <h2 className="px-4 pt-4 pb-2 text-sm font-medium">{title}</h2>
      <Table data-compact>
        <TableHeader>
          <TableRow>
            <TableHead>{t("fin.tx.category")}</TableHead>
            <TableHead className="w-24 text-right">{t("fin.report.count")}</TableHead>
            <TableHead className="w-40 text-right">{t("fin.tx.amount")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((c) => (
            <TableRow key={`${c.category_id}${c.kind}`}>
              <TableCell>
                {/* The transactions page filters one month; January opens the year. */}
                <Link className="hover:underline" href={`/transactions?category=${c.category_id || "none"}&month=${year}-01`}>{c.name || t("fin.uncategorized")}</Link>
                {isDeductible(c) && <span className="ml-2 rounded-full bg-sky-500/15 px-2 py-0.5 text-xs text-sky-800 dark:text-sky-300">{t("fin.report.deductible")}</span>}
              </TableCell>
              <TableCell className="text-right tabular-nums text-muted-foreground">{c.count}</TableCell>
              <TableCell className={cn("text-right font-mono tabular-nums", amountClass(c.total))}>{f.money(Math.abs(c.total))}</TableCell>
            </TableRow>
          ))}
          <TableRow>
            <TableCell className="font-medium">{t("fin.report.total")}</TableCell>
            <TableCell />
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
  const [params, set] = useListParams({ year: thisYear });
  const year = Number(params.year) || thisYear;
  const report = useQuery({ queryKey: ["finance", "year", year], queryFn: () => finance.year(year) });
  const r = report.data;
  const income = r?.categories.filter((c) => c.kind === "income" && c.total !== 0).sort((a, b) => b.total - a.total) ?? [];
  const expense = r?.categories.filter((c) => c.kind === "expense" && c.total !== 0).sort((a, b) => a.total - b.total) ?? [];
  const deductible = expense.filter(isDeductible).reduce((n, c) => n + c.total, 0);

  return (
    <>
      <PageHeader title={t("nav.report")} description={t("fin.report.subtitle")}
        action={
          <div className="flex items-center gap-1 print:hidden">
            <Button variant="ghost" size="icon" aria-label={t("fin.report.prev")} onClick={() => set({ year: year - 1 })}><ChevronLeft className="size-4" /></Button>
            <span className="min-w-14 text-center text-sm font-medium tabular-nums">{year}</span>
            <Button variant="ghost" size="icon" aria-label={t("fin.report.next")} disabled={year >= thisYear} onClick={() => set({ year: year + 1 })}><ChevronRight className="size-4" /></Button>
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
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Card className="gap-1 p-4"><span className="text-sm text-muted-foreground">{t("fin.report.income")}</span><span className="text-2xl font-semibold tabular-nums">{f.money(r.income)}</span></Card>
              <Card className="gap-1 p-4"><span className="text-sm text-muted-foreground">{t("fin.report.expense")}</span><span className="text-2xl font-semibold tabular-nums">{f.money(-r.expense)}</span></Card>
              <Card className="gap-1 p-4">
                <span className="text-sm text-muted-foreground">{t("fin.report.deductible_total")}</span>
                <span className="text-2xl font-semibold tabular-nums">{f.money(-deductible)}</span>
                <span className="text-xs text-muted-foreground">{t("fin.report.deductible_help")}</span>
              </Card>
            </div>
            <Card className="gap-3 p-4">
              <h2 className="text-sm font-medium">{t("fin.flow.title")}</h2>
              <MoneyFlow income={income} expense={expense} />
            </Card>
            <div className="grid gap-4 lg:grid-cols-2">
              <CategoryTable title={t("fin.report.income")} rows={income} total={r.income} year={year} />
              <CategoryTable title={t("fin.report.expense")} rows={expense} total={r.expense} year={year} />
            </div>
            <Card className="gap-0 overflow-hidden p-0">
              <h2 className="px-4 pt-4 pb-2 text-sm font-medium">{t("fin.report.by_month")}</h2>
              <Table data-compact>
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
            </Card>
            <p className="text-xs text-muted-foreground">{t("fin.report.note")}</p>
          </>
        )}
      </PageBody>
    </>
  );
}
