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

/** Months from today to a YYYY-MM-DD date, at least 1. */
function monthsUntil(date: string): number {
  const [y, m] = date.split("-").map(Number);
  const now = new Date();
  return Math.max(1, (y - now.getFullYear()) * 12 + (m - 1 - now.getMonth()));
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
function usePace(): number {
  const trend = useQuery({ queryKey: ["finance", "trend", 7], queryFn: () => finance.trend(7) });
  const done = (trend.data?.items ?? []).slice(0, -1); // the current month is still running
  const avg = done.length ? done.reduce((n, m) => n + m.income + m.expense, 0) / done.length : 0;
  return Math.max(0, Math.floor(avg));
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
      <p className="text-xs text-muted-foreground">
        {missing === 0
          ? t("fin.goal.reached")
          : g.due_date
            ? fill(t("fin.goal.per_month"), { v: f.money(Math.ceil(missing / monthsUntil(g.due_date))), d: f.date(g.due_date) })
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
  const pace = usePace();
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
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {items.map((g) => <GoalCard key={g.id} g={g} pace={pace} onEdit={() => setEditing({ goal: g })} onRemove={() => setRemoving(g)} />)}
          </div>
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
