"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Repeat, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { SubscriptionDialog, type Draft } from "@/components/subscription-dialog";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, type Recurring, type Subscription } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const KEY = ["finance", "subscriptions"];
/** A day of the month as YYYY-MM-DD, for the day/month formatter. */
const dayOf = (month: number, day: number) => `2000-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;

function SubscriptionRow({ s, onEdit, onRemove }: { s: Subscription; onEdit: () => void; onRemove: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const status = s.paid
    ? { text: fill(t("fin.sub.paid"), { d: f.day(s.last_paid) }), cls: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300" }
    : s.late
      ? { text: fill(t("fin.sub.late"), { d: f.day(s.next_due) }), cls: "bg-red-500/15 text-red-800 dark:text-red-300" }
      : { text: fill(t("fin.sub.due"), { d: f.day(s.next_due) }), cls: "bg-muted text-muted-foreground" };
  // A last charge 5% off the registered amount is worth seeing.
  const differs = s.last_paid && Math.abs(s.last_amount - s.amount) * 20 > s.amount;
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 text-sm">
      <div className="min-w-0 flex-1 max-sm:basis-full">
        <p className="truncate font-medium">{s.name}</p>
        <p className="text-xs text-muted-foreground">
          {s.month ? fill(t("fin.sub.every_year"), { d: f.day(dayOf(s.month, s.day)) }) : fill(t("fin.sub.every_month"), { d: s.day })}
          {differs && <span className="text-amber-800 dark:text-amber-300"> · {fill(t("fin.sub.last_charge"), { v: f.money(s.last_amount) })}</span>}
        </p>
      </div>
      <span className={cn("rounded-full px-2 py-0.5 text-xs font-medium", status.cls)}>{status.text}</span>
      <span className="w-28 text-right font-mono tabular-nums max-sm:ml-auto max-sm:w-auto">{f.money(s.amount)}</span>
      <div className="flex shrink-0 gap-1">
        <Button size="icon" variant="ghost" aria-label={`${t("fin.sub.edit")}: ${s.name}`} onClick={onEdit}><Pencil className="size-4" /></Button>
        <Button size="icon" variant="ghost" aria-label={`${t("fin.sub.remove")}: ${s.name}`} onClick={onRemove}><Trash2 className="size-4" /></Button>
      </div>
    </li>
  );
}

function DetectedRow({ r, onRegister }: { r: Recurring; onRegister: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const ignore = useMutation({
    mutationFn: () => finance.ignoreRecurring(r.key),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.sub.ignored"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 text-sm">
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{r.name}</p>
        <p className="text-xs text-muted-foreground">{fill(t("fin.sub.detected_seen"), { v: f.money(-r.amount), n: r.months })}</p>
      </div>
      <div className="flex shrink-0 gap-2">
        <Button size="sm" variant="ghost" disabled={ignore.isPending} onClick={() => ignore.mutate()}>{t("fin.sub.ignore")}</Button>
        <Button size="sm" variant="outline" onClick={onRegister}>{t("fin.sub.register")}</Button>
      </div>
    </li>
  );
}

export default function SubscriptionsPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const list = useQuery({ queryKey: KEY, queryFn: finance.subscriptions });
  const [editing, setEditing] = useState<Draft | null>(null);
  const [removing, setRemoving] = useState<Subscription | null>(null);
  const items = list.data?.items ?? [];
  const found = list.data?.detected ?? [];
  const blank: Draft = { name: "", amount: 0, day: 0, month: 0, keyword: "", category_id: "", account_id: "" };
  const add = <Button onClick={() => setEditing(blank)}><Plus className="size-4" />{t("fin.sub.new")}</Button>;
  const fromDetected = (r: Recurring): Draft => ({ ...blank, name: r.name, amount: -(r.changed ? r.last : r.amount), day: Number(r.last_date.slice(8, 10)), keyword: r.key });

  return (
    <>
      <PageHeader title={t("nav.subscriptions")} description={t("fin.sub.subtitle")} action={items.length > 0 && add} />
      <PageBody>
        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState error={list.error} onRetry={() => void list.refetch()} />
        ) : (
          <>
            {items.length === 0 ? (
              <EmptyState icon={Repeat} title={t("fin.sub.empty")} description={t("fin.sub.help")} action={add} />
            ) : (
              <>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Card className="gap-1 p-4">
                    <p className="text-sm text-muted-foreground">{t("fin.sub.monthly")}</p>
                    <p className="font-mono text-2xl font-semibold tabular-nums">{f.money(list.data.monthly)}</p>
                  </Card>
                  <Card className="gap-1 p-4">
                    <p className="text-sm text-muted-foreground">{t("fin.sub.yearly")}</p>
                    <p className="font-mono text-2xl font-semibold tabular-nums">{f.money(list.data.monthly * 12)}</p>
                  </Card>
                </div>
                <section className="grid gap-2">
                  <h2 className="text-sm font-medium">{t("fin.sub.registered")}</h2>
                  <Card className="gap-0 p-0">
                    <ul className="divide-y">
                      {items.map((s) => (
                        <SubscriptionRow key={s.id} s={s} onRemove={() => setRemoving(s)}
                          onEdit={() => setEditing({ id: s.id, name: s.name, amount: s.amount, day: s.day, month: s.month, keyword: s.keyword, category_id: s.category_id, account_id: s.account_id })} />
                      ))}
                    </ul>
                  </Card>
                </section>
              </>
            )}
            {found.length > 0 && (
              <section className="grid gap-2">
                <div>
                  <h2 className="text-sm font-medium">{t("fin.sub.detected")}</h2>
                  <p className="text-xs text-muted-foreground">{t("fin.sub.detected_help")}</p>
                </div>
                <Card className="gap-0 p-0">
                  <ul className="divide-y">
                    {found.map((r) => <DetectedRow key={r.key} r={r} onRegister={() => setEditing(fromDetected(r))} />)}
                  </ul>
                </Card>
              </section>
            )}
          </>
        )}
        {/* The key resets the form for each one opened. */}
        {editing && <SubscriptionDialog key={editing.id ?? editing.keyword ?? "new"} draft={editing} onClose={() => setEditing(null)} />}
        <ConfirmDialog
          open={!!removing}
          onOpenChange={(o) => !o && setRemoving(null)}
          title={fill(t("fin.sub.remove_title"), { n: removing?.name ?? "" })}
          destructive
          confirmLabel={t("fin.sub.remove")}
          onConfirm={async () => {
            if (!removing) return;
            await finance.removeSubscription(removing.id);
            setRemoving(null);
            await client.invalidateQueries({ queryKey: ["finance"] });
          }}
        />
      </PageBody>
    </>
  );
}
