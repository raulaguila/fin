"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Landmark } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { finance, monthKey, shiftMonth } from "@/lib/api/finance";
import { useFinanceFormat } from "@/lib/finance-format";
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
