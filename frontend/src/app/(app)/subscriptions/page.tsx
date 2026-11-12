"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Repeat, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useAccountLabel } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type Recurring, type Subscription, type SubscriptionInput } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const NONE = "none";
const KEY = ["finance", "subscriptions"];
const centsText = (c: number) => (c ? (c / 100).toFixed(2).replace(".", ",") : "");
/** A day of the month as YYYY-MM-DD, for the day/month formatter. */
const dayOf = (month: number, day: number) => `2000-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;

type Draft = SubscriptionInput & { id?: string };

function SubscriptionDialog({ draft, onClose }: { draft: Draft; onClose: () => void }) {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const label = useAccountLabel();
  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const [name, setName] = useState(draft.name);
  const [amount, setAmount] = useState(centsText(draft.amount));
  const [yearly, setYearly] = useState(draft.month > 0);
  const [day, setDay] = useState(String(draft.day || ""));
  const [month, setMonth] = useState(String(draft.month || new Date().getMonth() + 1));
  const [keyword, setKeyword] = useState(draft.keyword);
  const [categoryId, setCategoryId] = useState(draft.category_id || NONE);
  const [accountId, setAccountId] = useState(draft.account_id || NONE);
  const cents = parseCents(amount);
  const dayNum = Number(day);
  const valid = name.trim() !== "" && !!cents && cents > 0 && Number.isInteger(dayNum) && dayNum >= 1 && dayNum <= 31 && keyword.trim().length >= 2;
  const save = useMutation({
    mutationFn: () => {
      const body: SubscriptionInput = {
        name: name.trim(), amount: cents!, day: dayNum, month: yearly ? Number(month) : 0, keyword: keyword.trim(),
        category_id: categoryId === NONE ? "" : categoryId, account_id: accountId === NONE ? "" : accountId,
      };
      return draft.id ? finance.updateSubscription(draft.id, body) : finance.createSubscription(body);
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] }); // the forecast and the calendar use them too
      toast.success(t("fin.sub.saved"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });

  const frequencies = { monthly: t("fin.sub.monthly_option"), yearly: t("fin.sub.yearly_option") };
  const monthNames = new Intl.DateTimeFormat(locale, { month: "long", timeZone: "UTC" });
  const months: Record<string, string> = {};
  for (let m = 1; m <= 12; m++) months[String(m)] = monthNames.format(new Date(Date.UTC(2000, m - 1, 1)));
  const cats: Record<string, string> = { [NONE]: t("fin.sub.no_category") };
  for (const c of categories.data?.items ?? []) if (c.kind === "expense") cats[c.id] = c.name;
  const accounts: Record<string, string> = { [NONE]: t("fin.sub.no_account") };
  for (const c of connections.data?.items ?? []) for (const a of c.accounts) accounts[a.id] = `${c.institution} · ${label(a)}`;

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{draft.id ? t("fin.sub.edit") : t("fin.sub.new")}</DialogTitle>
          <DialogDescription>{t("fin.sub.help")}</DialogDescription>
        </DialogHeader>
        <form id="sub-form" className="grid gap-3" onSubmit={(e) => { e.preventDefault(); if (valid) save.mutate(); }}>
          <div className="grid grid-cols-[1fr_8rem] gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="sub-name">{t("fin.sub.name")}</Label>
              <Input id="sub-name" value={name} maxLength={60} onChange={(e) => setName(e.target.value)} placeholder={t("fin.sub.name_hint")} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="sub-amount">{t("fin.sub.amount")}</Label>
              <Input id="sub-amount" inputMode="decimal" className="text-right font-mono" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0,00" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label>{t("fin.sub.frequency")}</Label>
              <Select value={yearly ? "yearly" : "monthly"} items={frequencies} onValueChange={(v) => setYearly(v === "yearly")}>
                <SelectTrigger className="w-full" aria-label={t("fin.sub.frequency")}><SelectValue /></SelectTrigger>
                <SelectContent>{Object.entries(frequencies).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className={cn("grid gap-3", yearly && "grid-cols-[4.5rem_1fr]")}>
              <div className="grid gap-1.5">
                <Label htmlFor="sub-day">{t("fin.sub.day")}</Label>
                <Input id="sub-day" inputMode="numeric" className="font-mono" value={day} maxLength={2} onChange={(e) => setDay(e.target.value.replace(/\D/g, ""))} />
              </div>
              {yearly && (
                <div className="grid gap-1.5">
                  <Label>{t("fin.sub.month")}</Label>
                  <Select value={month} items={months} onValueChange={(v) => setMonth(String(v))}>
                    <SelectTrigger className="w-full" aria-label={t("fin.sub.month")}><SelectValue /></SelectTrigger>
                    <SelectContent>{Object.entries(months).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
                  </Select>
                </div>
              )}
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="sub-keyword">{t("fin.sub.keyword")}</Label>
            <Input id="sub-keyword" value={keyword} maxLength={60} onChange={(e) => setKeyword(e.target.value)} aria-describedby="sub-keyword-help" />
            <p id="sub-keyword-help" className="text-xs text-muted-foreground">{t("fin.sub.keyword_help")}</p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label>{t("fin.sub.category")}</Label>
              <Select value={categoryId} items={cats} onValueChange={(v) => setCategoryId(String(v ?? NONE))}>
                <SelectTrigger className="w-full" aria-label={t("fin.sub.category")}><SelectValue /></SelectTrigger>
                <SelectContent>{Object.entries(cats).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>{t("fin.sub.account")}</Label>
              <Select value={accountId} items={accounts} onValueChange={(v) => setAccountId(String(v ?? NONE))}>
                <SelectTrigger className="w-full" aria-label={t("fin.sub.account")}><SelectValue /></SelectTrigger>
                <SelectContent>{Object.entries(accounts).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
        </form>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button type="submit" form="sub-form" disabled={!valid || save.isPending}>{t("fin.sub.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

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
