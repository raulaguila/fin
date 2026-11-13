"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { HandCoins, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { PageBody, PageHeader } from "@/components/bits";
import { Debts } from "@/components/debts";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, type Person } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

/** Category the refund rules point to; created by default as an expense. */
const REFUNDS = "reembolsos";

function Row({ p, refundsId }: { p: Person; refundsId?: string }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const mark = useMutation({
    mutationFn: () => finance.createRule(p.keyword, refundsId ?? ""),
    onSuccess: (r) => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(fill(t("fin.people.refund_done"), { n: r.applied }));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const cell = (label: string, v: number) => (
    <div className="text-right">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={cn("font-mono tabular-nums", amountClass(v))}>{f.money(v)}</p>
    </div>
  );
  return (
    <li className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 py-3 text-sm">
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{p.name}</p>
        <p className="text-xs text-muted-foreground">{fill(t("fin.people.count"), { n: p.count, d: f.date(p.last_date) })}</p>
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        {p.sent !== 0 && cell(t("fin.people.sent"), p.sent)}
        {p.received !== 0 && cell(t("fin.people.received"), p.received)}
        {p.refunded !== 0 && cell(t("fin.people.refunded"), p.refunded)}
        {p.keyword && p.received > 0 && refundsId && (
          <Button size="sm" variant="outline" disabled={mark.isPending} onClick={() => mark.mutate()}>
            <Undo2 className="size-4" />{t("fin.people.mark_refund")}
          </Button>
        )}
      </div>
    </li>
  );
}

export default function PeoplePage() {
  const { t } = useI18n();
  const people = useQuery({ queryKey: ["finance", "people"], queryFn: finance.people });
  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories });
  const refundsId = categories.data?.items.find((c) => c.name.toLowerCase() === REFUNDS && c.kind === "expense")?.id;
  return (
    <>
      <PageHeader title={t("nav.people")} description={t("fin.people.subtitle")} />
      <PageBody>
        <Debts />
        {people.isPending ? (
          <ListSkeleton />
        ) : people.isError ? (
          <QueryErrorState error={people.error} onRetry={() => void people.refetch()} />
        ) : people.data.items.length === 0 ? (
          <EmptyState icon={HandCoins} title={t("nav.people")} description={t("fin.people.empty")} />
        ) : (
          <Card className="gap-2 p-4">
            <p className="text-xs text-muted-foreground">{refundsId ? t("fin.people.refund_help") : t("fin.people.no_refund_cat")}</p>
            <ul className="divide-y">{people.data.items.map((p) => <Row key={p.name} p={p} refundsId={refundsId} />)}</ul>
          </Card>
        )}
      </PageBody>
    </>
  );
}
