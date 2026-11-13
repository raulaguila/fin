"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Flag, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Meter, useAccountLabel } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { FormSheet } from "@/components/form-sheet";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type Goal } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { centsText, fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";

const NO_ACCOUNT = "none";

/** Months from today to a YYYY-MM-DD date, the current one included, at least 1 (as the goal alert counts). */
function monthsUntil(date: string): number {
  const [y, m] = date.split("-").map(Number);
  const now = new Date();
  return Math.max(1, (y - now.getFullYear()) * 12 + (m - 1 - now.getMonth()) + 1);
}

/** What a goal with a date asks for each month; 0 without a date or once reached. */
function monthlyNeed(g: Goal): number {
  const missing = Math.max(0, g.target - g.current);
  return g.due_date && missing > 0 ? Math.ceil(missing / monthsUntil(g.due_date)) : 0;
}

function GoalDialog({ goal, open, onClose }: { goal: Goal | null; open: boolean; onClose: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const label = useAccountLabel();
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const [name, setName] = useState(goal?.name ?? "");
  const [target, setTarget] = useState(centsText(goal?.target ?? 0));
  const [saved, setSaved] = useState(centsText(goal?.saved ?? 0));
  const [accountId, setAccountId] = useState(goal?.account_id || NO_ACCOUNT);
  const [due, setDue] = useState(goal?.due_date ?? "");
  const targetCents = parseCents(target), savedCents = parseCents(saved);
  const valid = name.trim() !== "" && !!targetCents && targetCents > 0 && savedCents !== null;
  const save = useMutation({
    mutationFn: () => {
      const body = { name: name.trim(), target: targetCents!, saved: savedCents ?? 0, account_id: accountId === NO_ACCOUNT ? "" : accountId, due_date: due };
      return goal ? finance.updateGoal(goal.id, body) : finance.createGoal(body);
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance", "goals"] });
      toast.success(t("fin.goal.saved"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const accounts: Record<string, string> = { [NO_ACCOUNT]: t("fin.goal.no_account") };
  for (const c of connections.data?.items ?? []) for (const a of c.accounts) if (a.type === "BANK") accounts[a.id] = `${c.institution} · ${label(a)}`;
  return (
    <FormSheet open={open} onClose={onClose} title={goal ? t("fin.goal.edit") : t("fin.goal.new")} description={t("fin.goal.help")}
      submitLabel={t("fin.goal.save")} canSubmit={valid} pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label htmlFor="goal-name">{t("fin.goal.name")}</Label>
        <Input id="goal-name" value={name} maxLength={60} onChange={(e) => setName(e.target.value)} placeholder={t("fin.goal.name_hint")} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="goal-target">{t("fin.goal.target")}</Label>
          <Input id="goal-target" inputMode="decimal" className="text-right font-mono" value={target} onChange={(e) => setTarget(e.target.value)} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="goal-due">{t("fin.goal.due")}</Label>
          <Input id="goal-due" type="date" value={due} onChange={(e) => setDue(e.target.value)} />
        </div>
      </div>
      <div className="grid gap-1.5">
        <Label>{t("fin.goal.account")}</Label>
        <Select value={accountId} items={accounts} onValueChange={(v) => setAccountId(String(v ?? NO_ACCOUNT))}>
          <SelectTrigger className="w-full" aria-label={t("fin.goal.account")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(accounts).map(([value, l]) => <SelectItem key={value} value={value}>{l}</SelectItem>)}</SelectContent>
        </Select>
      </div>
      {accountId === NO_ACCOUNT && (
        <div className="grid gap-1.5">
          <Label htmlFor="goal-saved">{t("fin.goal.saved_label")}</Label>
          <Input id="goal-saved" inputMode="decimal" className="text-right font-mono" value={saved} onChange={(e) => setSaved(e.target.value)} />
        </div>
      )}
    </FormSheet>
  );
}

/** Average monthly leftover (income less spending) of the complete months, or 0. */
/** What was left in the last full months: the average of six and the last one. */
function usePace(): { pace: number; last: number | null } {
  const trend = useQuery({ queryKey: ["finance", "trend", 7], queryFn: () => finance.trend(7) });
  const done = (trend.data?.items ?? []).slice(0, -1); // the current month is still running
  const avg = done.length ? done.reduce((n, m) => n + m.income + m.expense, 0) / done.length : 0;
  const last = done.at(-1);
  return { pace: Math.max(0, Math.floor(avg)), last: last ? last.income + last.expense : null };
}

/** The saving pace against what the goals with a date ask, so the flow of money and the goals meet. */
function PaceCard({ goals, pace, last }: { goals: Goal[]; pace: number; last: number | null }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const need = goals.reduce((n, g) => n + monthlyNeed(g), 0);
  const short = need - pace;
  return (
    <Card className="grid gap-3 p-4 sm:grid-cols-3">
      <div className="grid gap-0.5">
        <span className="text-xs text-muted-foreground">{t("fin.goal.last_month")}</span>
        <span className={`text-lg font-semibold tabular-nums ${last !== null && last < 0 ? "text-red-700 dark:text-red-400" : ""}`}>{last === null ? "—" : f.money(last)}</span>
      </div>
      <div className="grid gap-0.5">
        <span className="text-xs text-muted-foreground">{t("fin.goal.pace")}</span>
        <span className="text-lg font-semibold tabular-nums">{f.money(pace)}</span>
      </div>
      <div className="grid gap-0.5">
        <span className="text-xs text-muted-foreground">{t("fin.goal.need")}</span>
        <span className="text-lg font-semibold tabular-nums">{f.money(need)}</span>
      </div>
      {need > 0 && (
        <p className={`text-sm sm:col-span-3 ${short > 0 ? "text-amber-800 dark:text-amber-300" : "text-emerald-700 dark:text-emerald-400"}`}>
          {short > 0 ? fill(t("fin.goal.short"), { v: f.money(short) }) : t("fin.goal.on_pace_all")}
        </p>
      )}
    </Card>
  );
}

function GoalCard({ g, pace, onEdit, onRemove }: { g: Goal; pace: number; onEdit: () => void; onRemove: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const missing = Math.max(0, g.target - g.current);
  const pct = Math.min(100, Math.round((g.current / g.target) * 100));
  return (
    <Card className="gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate font-medium">{g.name}</h2>
          <p className="text-xs text-muted-foreground">{g.account_id ? t("fin.goal.linked") : t("fin.goal.manual")}</p>
        </div>
        <div className="flex shrink-0 gap-1">
          <Button size="icon" variant="ghost" aria-label={`${t("fin.goal.edit")}: ${g.name}`} onClick={onEdit}><Pencil className="size-4" /></Button>
          <Button size="icon" variant="ghost" aria-label={`${t("fin.goal.remove")}: ${g.name}`} onClick={onRemove}><Trash2 className="size-4" /></Button>
        </div>
      </div>
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <span className="font-mono tabular-nums">{f.money(g.current)} <span className="text-muted-foreground">/ {f.money(g.target)}</span></span>
        <span className="tabular-nums text-muted-foreground">{pct}%</span>
      </div>
      <Meter value={g.current} max={g.target} neutral />
      {monthlyNeed(g) > 0 && (
        <span className={`w-fit rounded-full px-2 py-0.5 text-xs ${monthlyNeed(g) <= pace ? "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300" : "bg-amber-500/15 text-amber-800 dark:text-amber-300"}`}>
          {monthlyNeed(g) <= pace ? t("fin.goal.on_pace") : t("fin.goal.behind")}
        </span>
      )}
      <p className="text-xs text-muted-foreground">
        {missing === 0
          ? t("fin.goal.reached")
          : g.due_date
            ? fill(t("fin.goal.per_month"), { v: f.money(monthlyNeed(g)), d: f.date(g.due_date) })
            : pace > 0
              ? fill(t("fin.goal.at_pace"), { v: f.money(missing), p: f.money(pace), n: Math.ceil(missing / pace) })
              : fill(t("fin.goal.missing"), { v: f.money(missing) })}
      </p>
    </Card>
  );
}

export default function GoalsPage() {
  const { t } = useI18n();
  const client = useQueryClient();
  const goals = useQuery({ queryKey: ["finance", "goals"], queryFn: finance.goals });
  const { pace, last } = usePace();
  const [editing, setEditing] = useState<{ goal: Goal | null } | null>(null);
  const [removing, setRemoving] = useState<Goal | null>(null);
  const items = goals.data?.items ?? [];
  const add = <Button onClick={() => setEditing({ goal: null })}><Plus className="size-4" />{t("fin.goal.new")}</Button>;
  return (
    <>
      <PageHeader title={t("nav.goals")} description={t("fin.goal.subtitle")} action={items.length > 0 && add} />
      <PageBody>
        {goals.isPending ? (
          <ListSkeleton />
        ) : goals.isError ? (
          <QueryErrorState error={goals.error} onRetry={() => void goals.refetch()} />
        ) : items.length === 0 ? (
          <EmptyState icon={Flag} title={t("fin.goal.empty")} description={t("fin.goal.help")} action={add} />
        ) : (
          <>
          <PaceCard goals={items} pace={pace} last={last} />
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {items.map((g) => <GoalCard key={g.id} g={g} pace={pace} onEdit={() => setEditing({ goal: g })} onRemove={() => setRemoving(g)} />)}
          </div>
          </>
        )}
        {/* The key resets the form for each goal opened. */}
        {editing && <GoalDialog key={editing.goal?.id ?? "new"} goal={editing.goal} open onClose={() => setEditing(null)} />}
        <ConfirmDialog
          open={!!removing}
          onOpenChange={(o) => !o && setRemoving(null)}
          title={fill(t("fin.goal.remove_title"), { n: removing?.name ?? "" })}
          destructive
          confirmLabel={t("fin.goal.remove")}
          onConfirm={async () => {
            if (!removing) return;
            await finance.removeGoal(removing.id);
            setRemoving(null);
            await client.invalidateQueries({ queryKey: ["finance", "goals"] });
          }}
        />
      </PageBody>
    </>
  );
}
