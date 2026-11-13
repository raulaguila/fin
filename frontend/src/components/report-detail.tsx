"use client";

import { useState, type KeyboardEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Loader2 } from "lucide-react";
import { CategoryDot } from "@/components/finance";
import { QueryErrorState } from "@/components/query-error-state";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { finance, type CategoryTotal, type KeyTotal } from "@/lib/api/finance";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { merchantName } from "@/lib/merchant";

export interface Series {
  label: string;
  color: string;
  /** Positive amounts by YYYY-MM. */
  values: Record<string, number>;
}

const BH = 150, AXIS = 18;

/** Twelve month slots of one or more series side by side; a click on a month calls onPick. */
/** width is the drawing's own width: smaller in a narrow panel keeps the month labels readable. */
export function MonthBars({ year, series, onPick, pickLabel, width: BW = 720 }: {
  year: number; series: Series[]; onPick?: (month: string) => void; pickLabel?: (month: string) => string; width?: number;
}) {
  const f = useFinanceFormat();
  const months = Array.from({ length: 12 }, (_, i) => `${year}-${String(i + 1).padStart(2, "0")}`);
  const max = Math.max(1, ...series.flatMap((s) => months.map((m) => s.values[m] ?? 0)));
  const slot = BW / 12;
  const bar = Math.min(18, (slot - 8) / series.length);
  const keys = (m: string) => (e: KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onPick?.(m); }
  };
  return (
    <div className="grid gap-2">
      {series.length > 1 && (
        <div className="flex flex-wrap gap-4 text-xs text-muted-foreground">
          {series.map((s) => <span key={s.label} className="flex items-center gap-1.5"><span className="size-2.5 rounded-sm" style={{ background: s.color }} />{s.label}</span>)}
        </div>
      )}
      <svg viewBox={`0 0 ${BW} ${BH + AXIS}`} className="h-auto w-full max-w-[900px]" role="img" aria-label={series.map((s) => s.label).join(", ")}>
        <line x1="0" x2={BW} y1={BH} y2={BH} className="stroke-border" />
        {months.map((m, i) => {
          const x0 = i * slot + (slot - bar * series.length - 2 * (series.length - 1)) / 2;
          const tip = `${f.month(m)}: ${series.map((s) => `${s.label} ${f.money(s.values[m] ?? 0)}`).join(" · ")}`;
          const pick = onPick ? { role: "link", tabIndex: 0, "aria-label": pickLabel?.(m) ?? tip, onClick: () => onPick(m), onKeyDown: keys(m), className: "group cursor-pointer outline-none" } : {};
          return (
            <g key={m} {...pick}>
              <title>{tip}</title>
              {/* The whole slot is the hit target, not only the bar. */}
              <rect x={i * slot} y="0" width={slot} height={BH} className="fill-transparent group-hover:fill-muted/60 group-focus-visible:fill-muted/60" />
              {series.map((s, j) => {
                const v = s.values[m] ?? 0;
                const h = v > 0 ? Math.max(2, (v / max) * (BH - 8)) : 0;
                return <rect key={s.label} x={x0 + j * (bar + 2)} y={BH - h} width={bar} height={h} rx="3" fill={s.color} />;
              })}
              <text x={i * slot + slot / 2} y={BH + 13} textAnchor="middle" className="fill-muted-foreground text-[11px]">{f.month(m).slice(0, 3)}</text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

/** Descriptions folded by merchant name, biggest first. */
function merchants(rows: KeyTotal[]) {
  const by = new Map<string, { name: string; total: number; count: number }>();
  for (const r of rows) {
    const name = merchantName(r.key);
    const m = by.get(name) ?? { name, total: 0, count: 0 };
    m.total += Math.abs(r.total);
    m.count += r.count;
    by.set(name, m);
  }
  return [...by.values()].sort((a, b) => b.total - a.total);
}

const TOP = 10;

function Detail({ year, month, cat }: { year: number; month?: string; cat: CategoryTotal }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const router = useRouter();
  const id = cat.category_id || "none";
  const detail = useQuery({
    queryKey: ["finance", "year", year, "category", id, cat.kind, month ?? ""],
    queryFn: () => finance.categoryDetail({ year, month, category_id: id, kind: cat.kind }),
  });
  if (detail.isPending) return <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />;
  if (detail.isError) return <QueryErrorState error={detail.error} onRetry={() => void detail.refetch()} />;

  const list = merchants(detail.data.descriptions);
  const shown = list.slice(0, TOP);
  const rest = list.slice(TOP);
  const max = shown[0]?.total || 1;
  const href = (m: string) => `/transactions?category=${id}&month=${m}`;
  const color = cat.kind === "income" ? "#1e9e73" : "#2a78d6";
  return (
    <>
      <div className="grid gap-0.5">
        <span className="text-2xl font-semibold tabular-nums">{f.money(Math.abs(cat.total))}</span>
        <span className="text-xs text-muted-foreground">{cat.count} {t("fin.report.count").toLowerCase()}</span>
      </div>
      {month ? (
        <Link href={href(month)} className="text-sm text-primary hover:underline">{fill(t("fin.detail.month_link"), { m: f.month(month) })}</Link>
      ) : (
        <section className="grid gap-2">
          <h3 className="text-sm font-medium">{t("fin.detail.by_month")}</h3>
          <MonthBars year={year} width={380} onPick={(m) => router.push(href(m))} pickLabel={(m) => fill(t("fin.detail.month_link"), { m: f.month(m) })}
            series={[{ label: cat.name || t("fin.uncategorized"), color, values: Object.fromEntries(detail.data.months.map((m) => [m.key, Math.abs(m.total)])) }]} />
        </section>
      )}
      <section className="grid gap-2">
        <h3 className="text-sm font-medium">{t("fin.detail.top")}</h3>
        <ol className="grid gap-2">
          {shown.map((m) => (
            <li key={m.name} className="grid gap-1">
              <div className="flex items-baseline justify-between gap-3 text-sm">
                <span className="min-w-0 truncate">{m.name}</span>
                <span className="shrink-0 font-mono tabular-nums">{f.money(m.total)} <span className="text-xs text-muted-foreground">{fill(t("fin.detail.times"), { n: m.count })}</span></span>
              </div>
              <div className="h-1.5 rounded-full bg-muted"><div className="h-1.5 rounded-full" style={{ width: `${(m.total / max) * 100}%`, background: color }} /></div>
            </li>
          ))}
        </ol>
        {rest.length > 0 && <p className="text-xs text-muted-foreground">{fill(t("fin.detail.more"), { n: rest.length, v: f.money(rest.reduce((n, m) => n + m.total, 0)) })}</p>}
      </section>
    </>
  );
}

/** What is behind a number of the report: a category, or the categories folded into one node. */
export function ReportDetail({ year, month, title, cats, onClose }: { year: number; month?: string; title: string; cats: CategoryTotal[]; onClose: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [picked, setPicked] = useState<CategoryTotal | null>(cats.length === 1 ? cats[0] : null);
  const group = cats.reduce((n, c) => n + Math.abs(c.total), 0) || 1;
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md">
        <SheetHeader className="border-b pr-12">
          <div className="flex items-center gap-2">
            {picked && cats.length > 1 && (
              <Button variant="ghost" size="icon" aria-label={t("common.back")} onClick={() => setPicked(null)}><ArrowLeft className="size-4" /></Button>
            )}
            <SheetTitle>{picked ? picked.name || t("fin.uncategorized") : title}</SheetTitle>
          </div>
          <SheetDescription>{month ? f.month(month) : year}</SheetDescription>
        </SheetHeader>
        <div className="grid flex-1 content-start gap-5 overflow-y-auto p-4">
          {picked ? (
            <Detail key={`${picked.category_id}${picked.kind}`} year={year} month={month} cat={picked} />
          ) : (
            <section className="grid gap-1">
              <h3 className="mb-1 text-sm font-medium">{t("fin.detail.members")}</h3>
              {cats.map((c) => (
                <button key={`${c.category_id}${c.kind}`} type="button" onClick={() => setPicked(c)}
                  className="flex items-center gap-3 rounded-md px-2 py-2 text-left text-sm hover:bg-muted">
                  <CategoryDot name={c.name} className="size-7" />
                  <span className="min-w-0 flex-1 truncate">{c.name || t("fin.uncategorized")}</span>
                  <span className="text-xs text-muted-foreground tabular-nums">{Math.round((Math.abs(c.total) / group) * 100)}%</span>
                  <span className="font-mono tabular-nums">{f.money(Math.abs(c.total))}</span>
                </button>
              ))}
            </section>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
