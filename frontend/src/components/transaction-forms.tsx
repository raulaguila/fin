"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Repeat, Split, Trash2, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { FormSheet } from "@/components/form-sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type Category, type Transaction } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { merchantName } from "@/lib/merchant";
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

const NONE = "none";

/** One transaction in full: category, note and tags, and what else can be done with it. */
export function TransactionSheet({ tx, categories, manual, onClose, onCategorized, onSplit, onUnsplit, onRemove, onFixed }: {
  tx: Transaction;
  categories: Category[];
  /** Typed or imported: it can be deleted. */
  manual: boolean;
  onClose: () => void;
  /** After a category change, with the keyword a rule would match. */
  onCategorized: (keyword: string, categoryId: string) => void;
  onSplit: () => void;
  /** Register this charge as a fixed monthly bill. */
  onFixed: () => void;
  onUnsplit: () => void;
  onRemove: () => void;
}) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [categoryId, setCategoryId] = useState(tx.category_id || NONE);
  const [note, setNote] = useState(tx.note ?? "");
  const [tags, setTags] = useState((tx.tags ?? []).join(", "));
  const picked = categoryId === NONE ? "" : categoryId;
  const changed = picked !== tx.category_id;
  const save = useMutation({
    mutationFn: () => finance.updateTransaction(tx.id, { ...(changed ? { category_id: picked } : {}), note: note.trim(), tags: splitTags(tags) }),
    onSuccess: ({ keyword }) => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.tx.details_saved"));
      if (changed && picked) onCategorized(keyword, picked);
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const cats: Record<string, string> = { [NONE]: t("fin.uncategorized") };
  for (const c of categories) cats[c.id] = c.name;
  return (
    <FormSheet onClose={onClose} title={merchantName(tx.description)}
      description={<>{tx.description}<span className="block tabular-nums">{f.date(tx.date)} · {tx.institution} · {tx.account_name} · <span className={amountClass(tx.amount)}>{f.money(tx.amount)}</span></span></>}
      submitLabel={t("fin.goal.save")} canSubmit pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label>{t("fin.tx.category")}</Label>
        <Select value={categoryId} items={cats} onValueChange={(v) => setCategoryId(String(v ?? NONE))}>
          <SelectTrigger className="w-full" aria-label={t("fin.tx.category")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(cats).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
        </Select>
        {tx.category_id && <p className="text-xs text-muted-foreground">{t(`fin.tx.source_${tx.category_source}`)}</p>}
      </div>
      <NoteField id="tx-note" value={note} onChange={setNote} />
      <TagsField id="tx-tags" value={tags} onChange={setTags} />
      <div className="flex flex-wrap gap-2 border-t pt-4">
        {tx.split_of ? (
          <Button type="button" variant="outline" size="sm" onClick={onUnsplit}><Undo2 className="size-4" />{t("fin.split.undo")}</Button>
        ) : (
          <Button type="button" variant="outline" size="sm" onClick={onSplit}><Split className="size-4" />{t("fin.split.action")}</Button>
        )}
        {tx.amount < 0 && <Button type="button" variant="outline" size="sm" onClick={onFixed}><Repeat className="size-4" />{t("fin.tx.fixed")}</Button>}
        {manual && <Button type="button" variant="outline" size="sm" onClick={onRemove}><Trash2 className="size-4" />{t("fin.tx.remove")}</Button>}
      </div>
    </FormSheet>
  );
}

const AUTO = "auto";
/** Picked when no imported account exists: a cash account is made on save. */
const WALLET = "wallet";

/** A transaction typed by hand, only on accounts with no bank behind them. */
export function NewTransactionSheet({ accounts, categories, onClose }: {
  accounts: Record<string, string>;
  categories: Category[];
  onClose: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const options = Object.keys(accounts).length ? accounts : { [WALLET]: t("fin.tx.wallet_new") };
  const [accountId, setAccountId] = useState(Object.keys(options)[0]);
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
    mutationFn: async () => finance.addTransaction({
      account_id: accountId === WALLET ? (await finance.createManualAccount({ institution: t("fin.tx.wallet_bank"), name: t("fin.tx.wallet"), type: "BANK" })).id : accountId,
      date, description: description.trim(), amount: kind === "out" ? -cents! : cents!,
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
        <Select value={accountId} items={options} onValueChange={(v) => setAccountId(String(v ?? ""))}>
          <SelectTrigger className="w-full" aria-label={t("fin.tx.account")}><SelectValue /></SelectTrigger>
          <SelectContent>{Object.entries(options).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
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
