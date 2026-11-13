"use client";

import { useState } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Meter, MonthSwitcher } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { finance, monthKey, shiftMonth, type CategoryTotal } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { centsText, fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

function LimitRow({ c }: { c: CategoryTotal }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  // limit already holds last month's carry; the field edits the category's own limit.
  const base = c.limit - c.carry;
  const [draft, setDraft] = useState(centsText(base));
  const save = useMutation({
    mutationFn: (patch: { limit?: number; rollover?: boolean }) => finance.updateCategory(c.category_id, patch),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.budget.saved"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const spent = -c.total;
  const ratio = c.limit > 0 ? spent / c.limit : 0;
  const status = c.limit <= 0 ? t("fin.budget.none") : ratio >= 1 ? t("fin.budget.over") : ratio >= 0.8 ? t("fin.budget.near") : t("fin.budget.ok");
  const commit = () => {
    const cents = parseCents(draft);
    if (cents === null || cents === base) return;
    save.mutate({ limit: cents });
  };
  return (
    <li className="grid grid-cols-1 gap-2 py-3 sm:grid-cols-[1fr_10rem] sm:items-center sm:gap-6">
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-sm">
          <span className="min-w-0 truncate font-medium">{c.name}</span>
          <span className="shrink-0 tabular-nums">
            <span className="font-mono">{f.money(spent)}</span>
            {c.limit > 0 && <span className="text-muted-foreground"> {fill(t("fin.budget.of_planned"), { v: f.money(c.limit) })}</span>}
          </span>
        </div>
        <Meter value={spent} max={c.limit} />
        <p className={cn("text-xs", ratio >= 1 ? "text-red-700 dark:text-red-400" : ratio >= 0.8 ? "text-amber-800 dark:text-amber-300" : "text-muted-foreground")}>
          {status}
          {c.carry !== 0 && <span className="text-muted-foreground"> · {fill(t(c.carry > 0 ? "fin.budget.carry_in" : "fin.budget.carry_out"), { v: f.money(Math.abs(c.carry)) })}</span>}
        </p>
      </div>
      <div className="grid gap-2">
        <label className="flex items-center gap-2 text-sm">
          <span className="whitespace-nowrap text-muted-foreground">{t("fin.budget.limit")} R$</span>
          <Input inputMode="decimal" value={draft} aria-invalid={parseCents(draft) === null} className="text-right font-mono"
            onChange={(e) => setDraft(e.target.value)} onBlur={commit} onKeyDown={(e) => e.key === "Enter" && commit()} />
        </label>
        {base > 0 && (
          <label className="flex items-center justify-end gap-2 text-xs text-muted-foreground" title={t("fin.budget.rollover_help")}>
            {t("fin.budget.rollover")}
            <Switch checked={c.rollover} disabled={save.isPending} onCheckedChange={(on) => save.mutate({ rollover: on })} aria-label={`${t("fin.budget.rollover")}: ${c.name}`} />
          </label>
        )}
      </div>
    </li>
  );
}

/** A category with no limit: its average spending of the last months, a click away from becoming the limit. */
function UnlimitedRow({ c, average }: { c: CategoryTotal; average: number }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [draft, setDraft] = useState("");
  const save = useMutation({
    mutationFn: (limit: number) => finance.updateCategory(c.category_id, { limit }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.budget.saved"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  // Rounded up to ten reais: a limit reads as a plan, not a statement.
  const suggested = Math.ceil(average / 1000) * 1000;
  const commit = () => {
    const cents = parseCents(draft);
    if (cents) save.mutate(cents);
  };
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 py-2 text-sm">
      <span className="min-w-0 flex-1">
        <span className="block truncate font-medium">{c.name}</span>
        <span className="text-xs text-muted-foreground">
          {average > 0 ? fill(t("fin.budget.average"), { v: f.money(average) }) : t("fin.budget.no_spend")}
          {c.total < 0 && ` · ${fill(t("fin.budget.this_month"), { v: f.money(-c.total) })}`}
        </span>
      </span>
      {suggested > 0 && (
        <Button size="sm" variant="outline" disabled={save.isPending} onClick={() => save.mutate(suggested)}>{fill(t("fin.budget.use"), { v: f.money(suggested) })}</Button>
      )}
      <Input inputMode="decimal" value={draft} placeholder={t("fin.budget.limit")} aria-label={`${t("fin.budget.limit")}: ${c.name}`} className="w-28 text-right font-mono"
        onChange={(e) => setDraft(e.target.value)} onBlur={commit} onKeyDown={(e) => e.key === "Enter" && commit()} />
    </li>
  );
}

export default function BudgetPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [params, set] = useListParams({ month: monthKey() });
  const summary = useQuery({ queryKey: ["finance", "summary", params.month], queryFn: () => finance.summary(params.month) });
  const rows = (summary.data?.categories ?? []).filter((c) => c.kind === "expense" && c.category_id);
  const limited = rows.filter((c) => c.limit - c.carry > 0);
  const unlimited = rows.filter((c) => c.limit - c.carry <= 0);
  // The three months before the one shown, for the average of each category.
  const past = useQueries({
    queries: [1, 2, 3].map((n) => {
      const m = shiftMonth(params.month, -n);
      return { queryKey: ["finance", "summary", m], queryFn: () => finance.summary(m), enabled: unlimited.length > 0 };
    }),
  });
  const average = (id: string) => {
    const months = past.filter((q) => q.data).map((q) => q.data!.categories.find((c) => c.category_id === id)?.total ?? 0);
    return months.length ? Math.round(-months.reduce((n, v) => n + v, 0) / months.length) : 0;
  };
  const [showAll, setShowAll] = useState(false);
  const spent = limited.reduce((n, c) => n - c.total, 0);
  const planned = limited.reduce((n, c) => n + c.limit, 0);
  const alerts = limited.filter((c) => -c.total >= c.limit * 0.8).length;

  const now = new Date();
  const current = params.month === monthKey(now);
  const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  const daysLeft = current ? lastDay - now.getDate() + 1 : 0;
  const left = planned - spent;

  return (
    <>
      <PageHeader title={t("nav.budget")}
        description={current ? `${t("fin.budget.subtitle")} ${fill(t("fin.budget.days_left"), { n: daysLeft })}.` : t("fin.budget.subtitle")}
        action={<MonthSwitcher month={params.month} onChange={(m) => set({ month: m })} />} />
      <PageBody>
        {summary.isPending ? (
          <ListSkeleton />
        ) : summary.isError ? (
          <QueryErrorState error={summary.error} onRetry={() => void summary.refetch()} />
        ) : (
          <>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
              <Card className="gap-1.5 p-4">
                <span className="text-sm text-muted-foreground">{t("fin.budget.spent")}</span>
                <span className="text-2xl font-semibold tabular-nums">{f.money(spent)}</span>
                <span className="text-xs text-muted-foreground">{fill(t("fin.budget.of_planned"), { v: f.money(planned) })}</span>
              </Card>
              <Card className="gap-1.5 p-4">
                <span className="text-sm text-muted-foreground">{t("fin.budget.can_spend")}</span>
                <span className={cn("text-2xl font-semibold tabular-nums", left < 0 && "text-red-700 dark:text-red-400")}>{f.money(left)}</span>
                {current && left > 0 && <span className="text-xs text-muted-foreground">{fill(t("fin.budget.per_day"), { v: f.money(Math.floor(left / daysLeft)) })}</span>}
              </Card>
              <Card className="gap-1.5 p-4">
                <span className="text-sm text-muted-foreground">{t("fin.budget.alerts")}</span>
                <span className="text-2xl font-semibold tabular-nums">{alerts}</span>
                <span className="text-xs text-muted-foreground">{t("fin.budget.alerts_hint")}</span>
              </Card>
            </div>
            <Card className="gap-1 p-4">
              <h2 className="text-sm font-medium">{t("fin.budget.limits")}</h2>
              <p className="text-xs text-muted-foreground">{limited.length ? t("fin.budget.limits_hint") : t("fin.budget.none_yet")}</p>
              <ul className="divide-y">
                {limited.map((c) => <LimitRow key={`${c.category_id}:${c.limit}:${c.carry}`} c={c} />)}
              </ul>
            </Card>
            {unlimited.length > 0 && (
              <Card className="gap-1 p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <h2 className="text-sm font-medium">{fill(t("fin.budget.unlimited"), { n: unlimited.length })}</h2>
                  <span className="text-xs text-muted-foreground">{t("fin.budget.unlimited_hint")}</span>
                </div>
                <ul className="divide-y">
                  {/* Where money goes first: the categories that spend the most lead. */}
                  {[...unlimited].sort((a, b) => average(b.category_id) - average(a.category_id))
                    .slice(0, showAll ? undefined : 5)
                    .map((c) => <UnlimitedRow key={c.category_id} c={c} average={average(c.category_id)} />)}
                </ul>
                {unlimited.length > 5 && (
                  <Button variant="ghost" size="sm" className="self-start" onClick={() => setShowAll(!showAll)}>
                    {showAll ? t("fin.budget.show_less") : fill(t("fin.budget.show_all"), { n: unlimited.length })}
                  </Button>
                )}
              </Card>
            )}
          </>
        )}
      </PageBody>
    </>
  );
}
