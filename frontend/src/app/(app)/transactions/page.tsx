"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search, Sparkles, Split, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { MonthSwitcher, NoConnections, useAccountLabel } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination } from "@/components/table-pagination";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { finance, monthKey, monthRange, type Category, type Transaction } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useDebounced } from "@/lib/hooks/use-debounced";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useI18n } from "@/lib/i18n/provider";
import { cn, searchTerm } from "@/lib/utils";

const NONE = "none";

/** The words a rule would match: the start of the description, without numbers. */
function suggestKeyword(description: string): string {
  const words = description.replace(/[0-9*#/\\-]+/g, " ").split(/\s+/).filter((w) => w.length > 1);
  return words.slice(0, 2).join(" ");
}

function CategoryPicker({ tx, categories, onPick }: { tx: Transaction; categories: Category[]; onPick: (tx: Transaction, id: string) => void }) {
  const { t } = useI18n();
  const items: Record<string, string> = { [NONE]: t("fin.uncategorized") };
  for (const c of categories) items[c.id] = c.name;
  return (
    <Select value={tx.category_id || NONE} items={items} onValueChange={(v) => onPick(tx, v === NONE ? "" : String(v))}>
      <SelectTrigger size="sm" className={cn("w-48 max-w-full", !tx.category_id && "text-muted-foreground")} aria-label={`${t("fin.tx.category")}: ${tx.description}`}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {Object.entries(items).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}

/** Moves part of a transaction to another category: a shared bill, a purchase with two uses. */
function SplitDialog({ tx, categories, onClose }: { tx: Transaction | null; categories: Category[]; onClose: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [amount, setAmount] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const cents = parseCents(amount);
  const whole = Math.abs(tx?.amount ?? 0);
  const valid = cents !== null && cents > 0 && cents < whole && categoryId !== "";
  const save = useMutation({
    mutationFn: () => finance.split(tx!.id, tx!.amount < 0 ? -cents! : cents!, categoryId),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.split.done"));
      close();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const close = () => { setAmount(""); setCategoryId(""); onClose(); };
  const items: Record<string, string> = {};
  for (const c of categories) items[c.id] = c.name;
  return (
    <Dialog open={!!tx} onOpenChange={(o) => !o && close()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("fin.split.title")}</DialogTitle>
          <DialogDescription>{tx && fill(t("fin.split.desc"), { d: tx.description, v: f.money(tx.amount) })}</DialogDescription>
        </DialogHeader>
        <form id="split-form" className="grid gap-3" onSubmit={(e) => { e.preventDefault(); if (valid) save.mutate(); }}>
          <div className="grid gap-1.5">
            <Label htmlFor="split-amount">{t("fin.split.amount")}</Label>
            <Input id="split-amount" inputMode="decimal" autoFocus value={amount} onChange={(e) => setAmount(e.target.value)}
              aria-invalid={amount !== "" && !valid && categoryId !== ""} className="text-right font-mono" placeholder="0,00" />
          </div>
          <div className="grid gap-1.5">
            <Label>{t("fin.split.category")}</Label>
            <Select value={categoryId} items={items} onValueChange={(v) => setCategoryId(String(v ?? ""))}>
              <SelectTrigger aria-label={t("fin.split.category")}><SelectValue placeholder={t("fin.split.category")} /></SelectTrigger>
              <SelectContent>{Object.entries(items).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
            </Select>
          </div>
          {cents !== null && cents > 0 && cents < whole && <p className="text-xs text-muted-foreground">{fill(t("fin.split.rest"), { v: f.money(whole - cents) })}</p>}
        </form>
        <DialogFooter>
          <Button variant="ghost" onClick={close}>{t("common.cancel")}</Button>
          <Button type="submit" form="split-form" disabled={!valid || save.isPending}>{t("fin.split.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function TransactionsPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [params, set] = useListParams({ page: 1, limit: 50, q: "", month: monthKey(), account: "", category: "" });
  const [search, setSearch] = useState(params.q);
  const [suggestion, setSuggestion] = useState<{ keyword: string; categoryId: string; categoryName: string } | null>(null);
  const [splitting, setSplitting] = useState<Transaction | null>(null);
  const q = searchTerm(useDebounced(params.q));
  const range = monthRange(params.month);

  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const list = useQuery({
    queryKey: ["finance", "transactions", params.page, params.limit, q, params.month, params.account, params.category],
    queryFn: ({ signal }) => finance.transactions({ page: params.page, limit: params.limit, search: q, ...range, account_id: params.account, category_id: params.category }, signal),
    placeholderData: keepPreviousData,
  });

  const setCategory = useMutation({
    mutationFn: ({ tx, categoryId }: { tx: Transaction; categoryId: string }) => finance.setCategory(tx.id, categoryId),
    onSuccess: (_, { tx, categoryId }) => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      const cat = categories.data?.items.find((c) => c.id === categoryId);
      const keyword = suggestKeyword(tx.description);
      setSuggestion(cat && keyword.length >= 2 ? { keyword, categoryId, categoryName: cat.name } : null);
      toast.success(t("fin.tx.saved"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const createRule = useMutation({
    mutationFn: (s: { keyword: string; categoryId: string }) => finance.createRule(s.keyword, s.categoryId),
    onSuccess: (r) => {
      setSuggestion(null);
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(fill(t("fin.tx.rule_created"), { n: r.applied }));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });

  const unsplit = useMutation({
    mutationFn: (originalId: string) => finance.unsplit(originalId),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.split.undone"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });

  const label = useAccountLabel();
  const accountItems: Record<string, string> = { "": t("fin.tx.all_accounts") };
  for (const c of connections.data?.items ?? []) for (const a of c.accounts) accountItems[a.id] = `${c.institution} · ${label(a)}`;
  const categoryItems: Record<string, string> = { "": t("fin.tx.all_categories"), [NONE]: t("fin.uncategorized") };
  for (const c of categories.data?.items ?? []) categoryItems[c.id] = c.name;

  const items = list.data?.items ?? [];
  const outflow = items.reduce((n, tx) => n + (tx.amount < 0 && tx.category_kind !== "transfer" ? tx.amount : 0), 0);
  const filtered = q || params.account || params.category;

  return (
    <>
      <PageHeader title={t("nav.transactions")} description={t("fin.tx.subtitle")} action={<MonthSwitcher month={params.month} onChange={(m) => set({ month: m, page: 1 })} />} />
      <PageBody>
        <div className="flex flex-wrap gap-2 max-sm:grid max-sm:grid-cols-1">
          <div className="relative min-w-56 flex-1 sm:max-w-sm max-sm:min-w-0">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input type="search" aria-label={t("common.search")} placeholder={t("fin.tx.search")} className="pl-9" value={search}
              onChange={(e) => { setSearch(e.target.value); set({ q: e.target.value.trim(), page: 1 }); }} />
          </div>
          <Select value={params.account} items={accountItems} onValueChange={(v) => set({ account: String(v ?? ""), page: 1 })}>
            <SelectTrigger className="w-56 max-sm:w-full" aria-label={t("fin.tx.all_accounts")}><SelectValue /></SelectTrigger>
            <SelectContent>{Object.entries(accountItems).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
          </Select>
          <Select value={params.category} items={categoryItems} onValueChange={(v) => set({ category: String(v ?? ""), page: 1 })}>
            <SelectTrigger className="w-56 max-sm:w-full" aria-label={t("fin.tx.all_categories")}><SelectValue /></SelectTrigger>
            <SelectContent>{Object.entries(categoryItems).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
          </Select>
        </div>

        {suggestion && (
          <Card className="flex-row flex-wrap items-center gap-3 p-3 text-sm" role="status">
            <Sparkles className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="min-w-0 flex-1">
              {t("fin.tx.rule_suggest")} <strong>“{suggestion.keyword}”</strong> → <strong>{suggestion.categoryName}</strong>
            </span>
            <Button size="sm" variant="ghost" onClick={() => setSuggestion(null)}>{t("fin.tx.rule_not_now")}</Button>
            <Button size="sm" disabled={createRule.isPending} onClick={() => createRule.mutate(suggestion)}>{t("fin.tx.rule_create")}</Button>
          </Card>
        )}

        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState error={list.error} onRetry={() => void list.refetch()} />
        ) : connections.data?.items.length === 0 ? (
          <NoConnections />
        ) : items.length === 0 ? (
          <EmptyState title={t("fin.tx.empty")}
            action={filtered && <Button variant="outline" onClick={() => { setSearch(""); set({ q: "", account: "", category: "", page: 1 }); }}>{t("common.clear_filters")}</Button>} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            <Table data-responsive data-compact>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-24">{t("fin.tx.date")}</TableHead>
                  <TableHead>{t("fin.tx.description")}</TableHead>
                  <TableHead className="w-52">{t("fin.tx.category")}</TableHead>
                  <TableHead className="w-36 text-right">{t("fin.tx.amount")}</TableHead>
                  <TableHead className="w-12"><span className="sr-only">{t("fin.split.action")}</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((tx) => (
                  <TableRow key={tx.id}>
                    <TableCell data-label={t("fin.tx.date")} data-cell="date" className="text-muted-foreground tabular-nums">{f.day(tx.date)}</TableCell>
                    <TableCell data-label={t("fin.tx.description")} data-cell="name" className="w-full max-w-0">
                      <p className="truncate font-medium" title={tx.description}>{tx.description}</p>
                      <p className="truncate text-xs text-muted-foreground">
                        {tx.institution} · {tx.account_name}
                        {tx.installments > 1 && ` · ${tx.installment}/${tx.installments}`}
                        {tx.status === "PENDING" && ` · ${t("fin.tx.pending")}`}
                        {tx.split_of && ` · ${t("fin.split.part")}`}
                      </p>
                    </TableCell>
                    <TableCell data-label={t("fin.tx.category")} data-cell="sub">
                      <CategoryPicker tx={tx} categories={categories.data?.items ?? []} onPick={(tx, categoryId) => setCategory.mutate({ tx, categoryId })} />
                    </TableCell>
                    <TableCell data-label={t("fin.tx.amount")} data-cell="sub" className={cn("text-right font-mono tabular-nums", amountClass(tx.amount))}>{f.money(tx.amount)}</TableCell>
                    <TableCell data-cell="sub" className="text-right">
                      {tx.split_of ? (
                        <Button size="icon" variant="ghost" aria-label={`${t("fin.split.undo")}: ${tx.description}`} title={t("fin.split.undo")}
                          disabled={unsplit.isPending} onClick={() => unsplit.mutate(tx.split_of)}><Undo2 className="size-4" /></Button>
                      ) : (
                        <Button size="icon" variant="ghost" aria-label={`${t("fin.split.action")}: ${tx.description}`} title={t("fin.split.action")}
                          onClick={() => setSplitting(tx)}><Split className="size-4" /></Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="border-t px-4 py-2 text-right text-sm text-muted-foreground">{fill(t("fin.tx.outflow"), { v: f.money(-outflow) })}</div>
            <TablePagination page={params.page} pageSize={params.limit} totalItems={list.data.pagination.total_items}
              onPageChange={(n) => set({ page: n })} onPageSizeChange={(n) => set({ limit: n, page: 1 })} />
          </Card>
        )}
        <SplitDialog tx={splitting} categories={categories.data?.items ?? []} onClose={() => setSplitting(null)} />
      </PageBody>
    </>
  );
}
