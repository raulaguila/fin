"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { FormSheet } from "@/components/form-sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type Category, type Transaction } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { parseCents } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";

/** "Viagem SP, casa" gives ["viagem-sp", "casa"]; the server checks the rest. */
export function splitTags(text: string): string[] {
  return [...new Set(text.split(",").map((s) => s.trim().toLowerCase().split(/\s+/).join("-")).filter(Boolean))];
}

function today(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function TagsField({ id, value, onChange }: { id: string; value: string; onChange: (v: string) => void }) {
  const { t } = useI18n();
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{t("fin.tx.tags")}</Label>
      <Input id={id} value={value} maxLength={400} placeholder={t("fin.tx.tags_hint")} onChange={(e) => onChange(e.target.value)} />
    </div>
  );
}

function NoteField({ id, value, onChange }: { id: string; value: string; onChange: (v: string) => void }) {
  const { t } = useI18n();
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{t("fin.tx.note")}</Label>
      <textarea id={id} value={value} maxLength={500} rows={3} onChange={(e) => onChange(e.target.value)}
        className="rounded-lg border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50" />
    </div>
  );
}

/** Note and tags of any transaction. */
export function DetailsSheet({ tx, onClose }: { tx: Transaction; onClose: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [note, setNote] = useState(tx.note ?? "");
  const [tags, setTags] = useState((tx.tags ?? []).join(", "));
  const save = useMutation({
    mutationFn: () => finance.setDetails(tx.id, note.trim(), splitTags(tags)),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance", "transactions"] });
      toast.success(t("fin.tx.details_saved"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <FormSheet onClose={onClose} title={t("fin.tx.details")} description={tx.description}
      submitLabel={t("fin.goal.save")} canSubmit pending={save.isPending} onSubmit={() => save.mutate()}>
      <NoteField id="tx-note" value={note} onChange={setNote} />
      <TagsField id="tx-tags" value={tags} onChange={setTags} />
    </FormSheet>
  );
}

const AUTO = "auto";

/** A transaction typed by hand, only on accounts with no bank behind them. */
export function NewTransactionSheet({ accounts, categories, onClose }: {
  accounts: Record<string, string>;
  categories: Category[];
  onClose: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [accountId, setAccountId] = useState(Object.keys(accounts)[0] ?? "");
  const [kind, setKind] = useState<"out" | "in">("out");
  const [date, setDate] = useState(today());
  const [description, setDescription] = useState("");
  const [amount, setAmount] = useState("");
  const [categoryId, setCategoryId] = useState(AUTO);
  const [note, setNote] = useState("");
  const [tags, setTags] = useState("");
  const cents = parseCents(amount);
  const valid = accountId !== "" && description.trim() !== "" && cents !== null && cents > 0 && date !== "";
  const save = useMutation({
    mutationFn: () => finance.addTransaction({
      account_id: accountId, date, description: description.trim(), amount: kind === "out" ? -cents! : cents!,
      category_id: categoryId === AUTO ? "" : categoryId, note: note.trim(), tags: splitTags(tags),
    }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.tx.added"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const kinds = { out: t("fin.tx.kind_out"), in: t("fin.tx.kind_in") };
  const cats: Record<string, string> = { [AUTO]: t("fin.tx.category_auto") };
  for (const c of categories) cats[c.id] = c.name;
  return (
    <FormSheet onClose={onClose} title={t("fin.tx.new")} description={t("fin.tx.new_help")}
      submitLabel={t("fin.goal.save")} canSubmit={valid} pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label>{t("fin.tx.account")}</Label>
        <Select value={accountId} items={accounts} onValueChange={(v) => setAccountId(String(v ?? ""))}>
          <SelectTrigger className="w-full" aria-label={t("fin.tx.account")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(accounts).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <Label>{t("fin.tx.kind")}</Label>
          <Select value={kind} items={kinds} onValueChange={(v) => setKind(v === "in" ? "in" : "out")}>
            <SelectTrigger className="w-full" aria-label={t("fin.tx.kind")}><SelectValue /></SelectTrigger>
            <SelectContent>{Object.entries(kinds).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="tx-date">{t("fin.tx.date")}</Label>
          <Input id="tx-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </div>
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="tx-desc">{t("fin.tx.description")}</Label>
        <Input id="tx-desc" autoFocus maxLength={200} value={description} onChange={(e) => setDescription(e.target.value)} />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="tx-amount">{t("fin.tx.amount")}</Label>
        <Input id="tx-amount" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} className="text-right font-mono" placeholder="0,00" />
      </div>
      <div className="grid gap-1.5">
        <Label>{t("fin.tx.category")}</Label>
        <Select value={categoryId} items={cats} onValueChange={(v) => setCategoryId(String(v ?? AUTO))}>
          <SelectTrigger className="w-full" aria-label={t("fin.tx.category")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(cats).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
        </Select>
      </div>
      <NoteField id="tx-new-note" value={note} onChange={setNote} />
      <TagsField id="tx-new-tags" value={tags} onChange={setTags} />
    </FormSheet>
  );
}
