"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Circle, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, monthKey, monthRange } from "@/lib/api/finance";
import { fill } from "@/lib/finance-format";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const HIDDEN = "fin.setup.hidden";

function hidden(): boolean {
  try {
    return localStorage.getItem(HIDDEN) === "1";
  } catch {
    return false;
  }
}

/** The four steps that make Fin useful, until they are done or the user hides them. */
export function SetupChecklist() {
  const { t } = useI18n();
  const [gone, setGone] = useState(hidden);
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections, enabled: !gone });
  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories, enabled: !gone });
  const goals = useQuery({ queryKey: ["finance", "goals"], queryFn: finance.goals, enabled: !gone });
  const month = monthKey();
  const loose = useQuery({
    queryKey: ["finance", "transactions", "uncategorized", month],
    queryFn: () => finance.transactions({ page: 1, limit: 1, ...monthRange(month), category_id: "none" }),
    select: (r) => r.pagination.total_items,
    enabled: !gone,
  });
  if (gone || !connections.data || !categories.data || !goals.data || loose.data === undefined) return null;

  const steps: { key: MessageKey; href: string; done: boolean }[] = [
    { key: "fin.setup.connect", href: "/connections", done: connections.data.items.length > 0 },
    { key: "fin.setup.review", href: "/transactions", done: loose.data === 0 },
    { key: "fin.setup.budget", href: "/budget", done: categories.data.items.some((c) => c.limit > 0) },
    { key: "fin.setup.goal", href: "/goals", done: goals.data.items.length > 0 },
  ];
  const done = steps.filter((s) => s.done).length;
  if (done === steps.length) return null;
  const hide = () => {
    try {
      localStorage.setItem(HIDDEN, "1");
    } catch {
      /* private mode: hidden for this visit only */
    }
    setGone(true);
  };
  return (
    <Card className="gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">{t("fin.setup.title")}</h2>
          <p className="text-xs text-muted-foreground">{fill(t("fin.setup.progress"), { n: done, total: steps.length })}</p>
        </div>
        <Button variant="ghost" size="icon" aria-label={t("fin.setup.hide")} title={t("fin.setup.hide")} onClick={hide}><X className="size-4" /></Button>
      </div>
      <div className="h-1.5 rounded-full bg-muted" aria-hidden="true">
        <div className="h-1.5 rounded-full bg-primary" style={{ width: `${(done / steps.length) * 100}%` }} />
      </div>
      <ol className="grid gap-1 sm:grid-cols-2">
        {steps.map((s) => (
          <li key={s.key}>
            <Link href={s.href} className={cn("flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted", s.done && "text-muted-foreground line-through")}>
              {s.done ? <CheckCircle2 className="size-4 shrink-0 text-emerald-600" aria-hidden="true" /> : <Circle className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />}
              {t(s.key)}
            </Link>
          </li>
        ))}
      </ol>
    </Card>
  );
}
