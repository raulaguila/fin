"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormSheet } from "@/components/form-sheet";
import { QueryErrorState } from "@/components/query-error-state";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type Debt, type DebtInput } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, centsText, fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

function today(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

const input = (d: Debt): DebtInput => ({ person: d.person, amount: d.amount, description: d.description, due_date: d.due_date, settled_on: d.settled_on });

function DebtSheet({ debt, onClose }: { debt: Debt | null; onClose: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [person, setPerson] = useState(debt?.person ?? "");
  const [side, setSide] = useState<"lent" | "borrowed">(debt && debt.amount < 0 ? "borrowed" : "lent");
  const [amount, setAmount] = useState(debt ? centsText(Math.abs(debt.amount)) : "");
  const [description, setDescription] = useState(debt?.description ?? "");
  const [due, setDue] = useState(debt?.due_date ?? "");
  const cents = parseCents(amount);
  const valid = person.trim() !== "" && cents !== null && cents > 0;
  const save = useMutation({
    mutationFn: () => {
      const body: DebtInput = { person: person.trim(), amount: side === "lent" ? cents! : -cents!, description: description.trim(), due_date: due, settled_on: debt?.settled_on ?? "" };
      return debt ? finance.updateDebt(debt.id, body) : finance.createDebt(body);
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance", "debts"] });
      toast.success(t("fin.debt.saved"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const sides = { lent: t("fin.debt.lent"), borrowed: t("fin.debt.borrowed") };
  return (
    <FormSheet onClose={onClose} title={debt ? t("fin.debt.edit") : t("fin.debt.new_title")}
      submitLabel={t("fin.goal.save")} canSubmit={valid} pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label>{t("fin.debt.side")}</Label>
        <Select value={side} items={sides} onValueChange={(v) => setSide(v === "borrowed" ? "borrowed" : "lent")}>
          <SelectTrigger className="w-full" aria-label={t("fin.debt.side")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(sides).map(([value, l]) => <SelectItem key={value} value={value}>{l}</SelectItem>)}</SelectContent>
        </Select>
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="debt-person">{t("fin.debt.person")}</Label>
        <Input id="debt-person" autoFocus maxLength={60} value={person} onChange={(e) => setPerson(e.target.value)} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="debt-amount">{t("fin.tx.amount")}</Label>
          <Input id="debt-amount" inputMode="decimal" className="text-right font-mono" placeholder="0,00" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="debt-due">{t("fin.goal.due")}</Label>
          <Input id="debt-due" type="date" value={due} onChange={(e) => setDue(e.target.value)} />
        </div>
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="debt-desc">{t("fin.debt.description")}</Label>
        <Input id="debt-desc" maxLength={120} value={description} onChange={(e) => setDescription(e.target.value)} />
      </div>
    </FormSheet>
  );
}

/** Who owes the user and whom the user owes, until marked paid. */
export function Debts() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const debts = useQuery({ queryKey: ["finance", "debts"], queryFn: finance.debts });
  const [editing, setEditing] = useState<Debt | null | "new">(null);
  const [removing, setRemoving] = useState<Debt | null>(null);
  const settle = useMutation({
    mutationFn: (d: Debt) => finance.updateDebt(d.id, { ...input(d), settled_on: d.settled_on ? "" : today() }),
    onSuccess: () => void client.invalidateQueries({ queryKey: ["finance", "debts"] }),
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const items = debts.data?.items ?? [];
  const open = items.filter((d) => !d.settled_on);
  const owed = open.reduce((n, d) => n + (d.amount > 0 ? d.amount : 0), 0);
  const owe = open.reduce((n, d) => n + (d.amount < 0 ? -d.amount : 0), 0);
  return (
    <Card className="gap-3 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="font-medium">{t("fin.debt.title")}</h2>
          <p className="text-xs text-muted-foreground tabular-nums">{fill(t("fin.debt.totals"), { a: f.money(owed), b: f.money(owe) })}</p>
        </div>
        <Button size="sm" onClick={() => setEditing("new")}><Plus className="size-4" />{t("fin.debt.new")}</Button>
      </div>
      {debts.isError ? (
        <QueryErrorState error={debts.error} onRetry={() => void debts.refetch()} />
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("fin.debt.empty")}</p>
      ) : (
        <ul className="divide-y">
          {items.map((d) => (
            <li key={d.id} className={cn("flex flex-wrap items-center gap-x-4 gap-y-1 py-2 text-sm", d.settled_on && "text-muted-foreground")}>
              <div className="min-w-0 flex-1">
                <p className={cn("truncate font-medium", d.settled_on && "line-through")}>{d.person}{d.description && <span className="font-normal text-muted-foreground"> · {d.description}</span>}</p>
                <p className="text-xs text-muted-foreground">
                  {d.amount > 0 ? t("fin.debt.owes_you") : t("fin.debt.you_owe")}
                  {d.settled_on ? ` · ${fill(t("fin.debt.paid_on"), { d: f.date(d.settled_on) })}` : d.due_date && ` · ${fill(t("fin.debt.due_on"), { d: f.date(d.due_date) })}`}
                </p>
              </div>
              <span className={cn("font-mono tabular-nums", !d.settled_on && amountClass(d.amount))}>{f.money(Math.abs(d.amount))}</span>
              <div className="flex shrink-0">
                <Button size="icon" variant="ghost" aria-label={`${d.settled_on ? t("fin.debt.reopen") : t("fin.debt.settle")}: ${d.person}`}
                  title={d.settled_on ? t("fin.debt.reopen") : t("fin.debt.settle")} disabled={settle.isPending} onClick={() => settle.mutate(d)}>
                  <Check className={cn("size-4", d.settled_on && "text-emerald-600")} />
                </Button>
                <Button size="icon" variant="ghost" aria-label={`${t("fin.debt.edit")}: ${d.person}`} onClick={() => setEditing(d)}><Pencil className="size-4" /></Button>
                <Button size="icon" variant="ghost" aria-label={`${t("fin.debt.remove")}: ${d.person}`} onClick={() => setRemoving(d)}><Trash2 className="size-4" /></Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {editing && <DebtSheet key={editing === "new" ? "new" : editing.id} debt={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
      <ConfirmDialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)} destructive
        title={fill(t("fin.debt.remove_title"), { n: removing?.person ?? "" })} confirmLabel={t("fin.debt.remove")}
        onConfirm={async () => {
          await finance.removeDebt(removing!.id);
          void client.invalidateQueries({ queryKey: ["finance", "debts"] });
        }} />
    </Card>
  );
}
