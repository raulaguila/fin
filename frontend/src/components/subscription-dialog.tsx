"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useAccountLabel } from "@/components/finance";
import { FormSheet } from "@/components/form-sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, type SubscriptionInput } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { centsText, parseCents } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";

const NONE = "none";

export type Draft = SubscriptionInput & { id?: string };

/** Registers or edits a fixed charge; also opened from a transaction. */
export function SubscriptionDialog({ draft, onClose }: { draft: Draft; onClose: () => void }) {
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

  const pick = (label: string, value: string, items: Record<string, string>, set: (v: string) => void) => (
    <div className="grid gap-1.5">
      <Label>{label}</Label>
      <Select value={value} items={items} onValueChange={(v) => set(String(v ?? NONE))}>
        <SelectTrigger className="w-full" aria-label={label}><SelectValue /></SelectTrigger>
        <SelectContent>{Object.entries(items).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
      </Select>
    </div>
  );

  return (
    <FormSheet onClose={onClose} title={draft.id ? t("fin.sub.edit") : t("fin.sub.new")} description={t("fin.sub.help")}
      submitLabel={t("fin.sub.save")} canSubmit={valid} pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label htmlFor="sub-name">{t("fin.sub.name")}</Label>
        <Input id="sub-name" value={name} maxLength={60} onChange={(e) => setName(e.target.value)} placeholder={t("fin.sub.name_hint")} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="sub-amount">{t("fin.sub.amount")}</Label>
          <Input id="sub-amount" inputMode="decimal" className="text-right font-mono" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0,00" />
        </div>
        {pick(t("fin.sub.frequency"), yearly ? "yearly" : "monthly", frequencies, (v) => setYearly(v === "yearly"))}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="sub-day">{t("fin.sub.day")}</Label>
          <Input id="sub-day" inputMode="numeric" className="font-mono" value={day} maxLength={2} onChange={(e) => setDay(e.target.value.replace(/\D/g, ""))} />
        </div>
        {yearly && pick(t("fin.sub.month"), month, months, setMonth)}
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="sub-keyword">{t("fin.sub.keyword")}</Label>
        <Input id="sub-keyword" value={keyword} maxLength={60} onChange={(e) => setKeyword(e.target.value)} aria-describedby="sub-keyword-help" />
        <p id="sub-keyword-help" className="text-xs text-muted-foreground">{t("fin.sub.keyword_help")}</p>
      </div>
      {pick(t("fin.sub.category"), categoryId, cats, setCategoryId)}
      {pick(t("fin.sub.account"), accountId, accounts, setAccountId)}
    </FormSheet>
  );
}
