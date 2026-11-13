"use client";

import { useState } from "react";
import type { CategoryTotal } from "@/lib/api/finance";
import { useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { categoryColor } from "@/lib/merchant";

interface Node {
  name: string;
  value: number;
  color: string;
}

const W = 900, PAD = 170, NODE = 12, GAP = 8, SLOT = 32;

/** Keeps the biggest n and folds the rest into one node, so labels never pile up. */
function top(nodes: Node[], n: number, rest: string): Node[] {
  if (nodes.length <= n) return nodes;
  const kept = nodes.slice(0, n - 1);
  const folded = nodes.slice(n - 1).reduce((s, x) => s + x.value, 0);
  return [...kept, { name: rest, value: folded, color: "var(--muted-foreground)" }];
}

/** Where the year's money came from and where it went: income on the left, spending and savings on the right. */
export function MoneyFlow({ income, expense }: { income: CategoryTotal[]; expense: CategoryTotal[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const [hover, setHover] = useState<string | null>(null);
  const totalIn = income.reduce((n, c) => n + Math.max(0, c.total), 0);
  const totalOut = expense.reduce((n, c) => n + Math.max(0, -c.total), 0);
  if (totalIn <= 0 || totalOut <= 0) return null;

  const IN = "#1e9e73";
  let left = top(income.filter((c) => c.total > 0).map((c) => ({ name: c.name || t("fin.uncategorized"), value: c.total, color: IN })), 6, t("fin.flow.other_in"));
  let right = top(expense.filter((c) => c.total < 0).map((c) => ({ name: c.name || t("fin.uncategorized"), value: -c.total, color: categoryColor(c.name) })), 8, t("fin.flow.other_out"));
  // Spent less than earned: the rest was kept. Spent more: it came out of the balance.
  if (totalIn > totalOut) right = [...right, { name: t("fin.flow.kept"), value: totalIn - totalOut, color: IN }];
  if (totalOut > totalIn) left = [...left, { name: t("fin.flow.from_balance"), value: totalOut - totalIn, color: "#d9a21b" }];

  const total = Math.max(totalIn, totalOut);
  // Bands follow the money; each node still gets room for its two label lines.
  const scale = 300 / total;
  const place = (nodes: Node[]) => {
    let y = 0;
    return nodes.map((n) => {
      const h = Math.max(2, n.value * scale);
      const out = { ...n, y, h };
      y += Math.max(h, SLOT) + GAP;
      return out;
    });
  };
  const L = place(left), R = place(right);
  const end = (nodes: { y: number; h: number }[]) => nodes.reduce((m, n) => Math.max(m, n.y + Math.max(n.h, SLOT)), 0);
  const H = Math.max(end(L), end(R));
  const cx = W / 2 - NODE / 2;
  const centerH = total * scale;
  const centerY = (H - centerH) / 2;

  const band = (x0: number, y0: number, x1: number, y1: number, h: number) => {
    const m = (x0 + x1) / 2;
    return `M${x0},${y0}C${m},${y0} ${m},${y1} ${x1},${y1}L${x1},${y1 + h}C${m},${y1 + h} ${m},${y0 + h} ${x0},${y0 + h}Z`;
  };
  // Where each band meets the center node, stacked in order.
  const at = (nodes: { h: number }[]) => nodes.map((_, i) => centerY + nodes.slice(0, i).reduce((n, x) => n + x.h, 0));
  const inAt = at(L), outAt = at(R);
  const label = (n: Node) => `${n.name}: ${f.money(n.value)}`;

  return (
    <div className="overflow-x-auto">
      <svg viewBox={`0 0 ${W} ${H}`} className="h-auto w-full min-w-[640px]" role="img" aria-label={t("fin.flow.title")}>
        {L.map((n, i) => {
          const d = band(PAD + NODE, n.y, cx, inAt[i], n.h);
          return <path key={`l${n.name}`} d={d} fill={n.color} opacity={hover && hover !== n.name ? 0.12 : 0.35}
            onPointerEnter={() => setHover(n.name)} onPointerLeave={() => setHover(null)}><title>{label(n)}</title></path>;
        })}
        {R.map((n, i) => {
          const d = band(cx + NODE, outAt[i], W - PAD - NODE, n.y, n.h);
          return <path key={`r${n.name}`} d={d} fill={n.color} opacity={hover && hover !== n.name ? 0.12 : 0.35}
            onPointerEnter={() => setHover(n.name)} onPointerLeave={() => setHover(null)}><title>{label(n)}</title></path>;
        })}
        <rect x={cx} y={centerY} width={NODE} height={centerH} rx="3" className="fill-foreground/70" />
        {L.map((n) => (
          <g key={`ln${n.name}`}>
            <rect x={PAD} y={n.y} width={NODE} height={n.h} rx="3" fill={n.color} />
            <text x={PAD - 8} y={n.y + n.h / 2} dy="0.35em" textAnchor="end" className="fill-foreground text-[12px]">{n.name}</text>
            <text x={PAD - 8} y={n.y + n.h / 2 + 14} dy="0.35em" textAnchor="end" className="fill-muted-foreground text-[11px] tabular-nums">{f.money(n.value)}</text>
          </g>
        ))}
        {R.map((n) => (
          <g key={`rn${n.name}`}>
            <rect x={W - PAD - NODE} y={n.y} width={NODE} height={n.h} rx="3" fill={n.color} />
            <text x={W - PAD + 8} y={n.y + n.h / 2} dy="0.35em" className="fill-foreground text-[12px]">{n.name}</text>
            <text x={W - PAD + 8} y={n.y + n.h / 2 + 14} dy="0.35em" className="fill-muted-foreground text-[11px] tabular-nums">{f.money(n.value)}</text>
          </g>
        ))}
      </svg>
    </div>
  );
}
