"use client";

import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowDownLeft, CalendarDays, CreditCard, Repeat, type LucideIcon } from "lucide-react";
import { NoConnections } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Card } from "@/components/ui/card";
import { finance, type CalendarEvent } from "@/lib/api/finance";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const ICONS: Record<CalendarEvent["kind"], LucideIcon> = { bill: CreditCard, recurring: Repeat, income: ArrowDownLeft };

export default function CalendarPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const cal = useQuery({ queryKey: ["finance", "calendar"], queryFn: finance.calendar });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const c = cal.data;
  const firstNegative = c?.events.find((e) => e.balance < 0);

  return (
    <>
      <PageHeader title={t("nav.calendar")} description={t("fin.cal.subtitle")} />
      <PageBody>
        {cal.isPending ? (
          <ListSkeleton />
        ) : cal.isError ? (
          <QueryErrorState error={cal.error} onRetry={() => void cal.refetch()} />
        ) : connections.data?.items.length === 0 ? (
          <NoConnections />
        ) : (
          c && (
            <>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Card className="gap-1 p-4"><span className="text-sm text-muted-foreground">{t("fin.cal.start")}</span><span className="text-2xl font-semibold tabular-nums">{f.money(c.start)}</span></Card>
                <Card className="gap-1 p-4">
                  <span className="text-sm text-muted-foreground">{t("fin.cal.lowest")}</span>
                  <span className={cn("text-2xl font-semibold tabular-nums", c.lowest < 0 && "text-red-700 dark:text-red-400")}>{f.money(c.lowest)}</span>
                </Card>
              </div>
              {firstNegative && (
                <Card className="flex-row items-center gap-3 border-red-500/40 bg-red-500/10 p-4 text-sm" role="status">
                  <AlertTriangle className="size-4 shrink-0 text-red-700 dark:text-red-400" aria-hidden="true" />
                  {fill(t("fin.cal.negative"), { d: f.date(firstNegative.date), v: f.money(firstNegative.balance) })}
                </Card>
              )}
              {c.events.length === 0 ? (
                <EmptyState icon={CalendarDays} title={t("fin.cal.empty")} />
              ) : (
                <Card className="gap-0 p-0">
                  <ul className="divide-y">
                    {c.events.map((e, i) => {
                      const Icon = ICONS[e.kind];
                      return (
                        <li key={`${e.date}${e.name}${i}`} className="flex items-center gap-3 px-4 py-3 text-sm">
                          <span className="w-14 shrink-0 tabular-nums text-muted-foreground">{f.day(e.date)}</span>
                          <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                          <div className="min-w-0 flex-1">
                            <p className="truncate font-medium">{e.name}</p>
                            <p className="text-xs text-muted-foreground">{t(`fin.cal.kind.${e.kind}` as MessageKey)}</p>
                          </div>
                          <div className="shrink-0 text-right">
                            <p className={cn("font-mono tabular-nums", amountClass(e.amount))}>{f.money(e.amount)}</p>
                            <p className={cn("text-xs font-mono tabular-nums text-muted-foreground", e.balance < 0 && "text-red-700 dark:text-red-400")}>
                              {fill(t("fin.cal.balance"), { v: f.money(e.balance) })}
                            </p>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                </Card>
              )}
              <p className="text-xs text-muted-foreground">{t("fin.cal.note")}</p>
            </>
          )
        )}
      </PageBody>
    </>
  );
}
