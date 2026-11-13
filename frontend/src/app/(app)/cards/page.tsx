"use client";

import { useQuery } from "@tanstack/react-query";
import { CreditCard } from "lucide-react";
import { NoConnections } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { finance, type CardBill } from "@/lib/api/finance";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";

/** What the installments already bought take from the months ahead. */
function Future({ items }: { items: { month: string; amount: number }[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const top = Math.max(1, ...items.map((m) => -m.amount));
  const total = items.reduce((n, m) => n - m.amount, 0);
  return (
    <Card className="gap-3 p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-sm font-medium">{t("fin.cards.future")}</h2>
        <span className="text-sm tabular-nums text-muted-foreground">{fill(t("fin.cards.future_total"), { v: f.money(total) })}</span>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("fin.cards.future_none")}</p>
      ) : (
        <ul className="space-y-1.5">
          {items.map((m) => (
            <li key={m.month} className="grid grid-cols-[9.5rem_1fr_6.5rem] max-sm:grid-cols-[7.5rem_1fr_5.5rem] items-center gap-3 text-sm">
              <span className="truncate text-muted-foreground first-letter:uppercase">{f.month(m.month)}</span>
              <span className="h-2 rounded-full bg-muted" aria-hidden="true">
                <span className="block h-2 rounded-full bg-[#eb6834] dark:bg-[#d95926]" style={{ width: `${(-m.amount / top) * 100}%` }} />
              </span>
              <span className="text-right font-mono tabular-nums">{f.money(-m.amount)}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function Bills({ name, bills }: { name: string; bills: CardBill[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  return (
    <Card className="gap-0 overflow-hidden p-0">
      <h2 className="flex items-center gap-2 border-b px-4 py-3 text-sm font-medium"><CreditCard className="size-4 text-muted-foreground" aria-hidden="true" />{name}</h2>
      <Table data-responsive data-compact>
        <TableHeader>
          <TableRow>
            <TableHead>{t("fin.cards.close")}</TableHead>
            <TableHead>{t("fin.cards.due")}</TableHead>
            <TableHead className="text-right">{t("fin.cards.charges")}</TableHead>
            <TableHead className="text-right">{t("fin.cards.credits")}</TableHead>
            <TableHead className="text-right">{t("fin.cards.count")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {bills.map((b) => (
            <TableRow key={b.close}>
              <TableCell data-label={t("fin.cards.close")} className="tabular-nums">{f.date(b.close)}</TableCell>
              <TableCell data-label={t("fin.cards.due")} className="tabular-nums text-muted-foreground">{b.due ? f.date(b.due) : "—"}</TableCell>
              <TableCell data-label={t("fin.cards.charges")} className="text-right font-mono tabular-nums">{f.money(-b.charges)}</TableCell>
              <TableCell data-label={t("fin.cards.credits")} className="text-right font-mono tabular-nums text-muted-foreground">{b.credits ? f.money(b.credits) : "—"}</TableCell>
              <TableCell data-label={t("fin.cards.count")} className="text-right tabular-nums text-muted-foreground">{b.count}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  );
}

export default function CardsPage() {
  const { t } = useI18n();
  const cards = useQuery({ queryKey: ["finance", "cards"], queryFn: finance.cards });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const byCard = new Map<string, CardBill[]>();
  for (const b of cards.data?.bills ?? []) byCard.set(b.account_id, [...(byCard.get(b.account_id) ?? []), b]);
  return (
    <>
      <PageHeader title={t("nav.cards")} description={t("fin.cards.subtitle")} />
      <PageBody>
        {cards.isPending ? (
          <ListSkeleton />
        ) : cards.isError ? (
          <QueryErrorState error={cards.error} onRetry={() => void cards.refetch()} />
        ) : connections.data?.items.length === 0 ? (
          <NoConnections />
        ) : byCard.size === 0 ? (
          <EmptyState icon={CreditCard} title={t("nav.cards")} description={t("fin.cards.empty")} />
        ) : (
          <>
            <Future items={cards.data.future} />
            {[...byCard].map(([id, bills]) => <Bills key={id} name={bills[0].account_name} bills={bills} />)}
          </>
        )}
      </PageBody>
    </>
  );
}
