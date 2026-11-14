"use client";

import { useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, ListChecks, Plus, Search, Sparkles, X } from "lucide-react";
import { toast } from "sonner";
import { CategoryDot, MonthSwitcher, NoConnections, useAccountLabel } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { FormSheet } from "@/components/form-sheet";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination } from "@/components/table-pagination";
import { ReviewSheet } from "@/components/review-sheet";
import { SubscriptionDialog, type Draft } from "@/components/subscription-dialog";
import { SelectAllHead, SelectCell, SelectionBar, useSelection } from "@/components/selection";
import { NewTransactionSheet, TransactionSheet } from "@/components/transaction-forms";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { finance, monthKey, monthRange, type Category, type Transaction } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, fill, parseCents, useFinanceFormat } from "@/lib/finance-format";
import { useDebounced } from "@/lib/hooks/use-debounced";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useUndoToast } from "@/lib/hooks/use-undo-toast";
import { useI18n } from "@/lib/i18n/provider";
import { repeated } from "@/lib/duplicates";
import { merchantName } from "@/lib/merchant";
import { cn, searchTerm } from "@/lib/utils";

const NONE = "none";

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
    <FormSheet open={!!tx} onClose={close} title={t("fin.split.title")}
      description={tx && fill(t("fin.split.desc"), { d: tx.description, v: f.money(tx.amount) })}
      submitLabel={t("fin.split.save")} canSubmit={valid} pending={save.isPending} onSubmit={() => save.mutate()}>
      <div className="grid gap-1.5">
        <Label htmlFor="split-amount">{t("fin.split.amount")}</Label>
        <Input id="split-amount" inputMode="decimal" autoFocus value={amount} onChange={(e) => setAmount(e.target.value)}
          aria-invalid={amount !== "" && !valid && categoryId !== ""} className="text-right font-mono" placeholder="0,00" />
        {cents !== null && cents > 0 && cents < whole && <p className="text-xs text-muted-foreground">{fill(t("fin.split.rest"), { v: f.money(whole - cents) })}</p>}
      </div>
      <div className="grid gap-1.5">
        <Label>{t("fin.split.category")}</Label>
        <Select value={categoryId} items={items} onValueChange={(v) => setCategoryId(String(v ?? ""))}>
          <SelectTrigger className="w-full" aria-label={t("fin.split.category")}><SelectValue placeholder={t("fin.split.category")} /></SelectTrigger>
          <SelectContent>{Object.entries(items).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
        </Select>
      </div>
    </FormSheet>
  );
}

/** The line under a transaction's name: where it came from and what it is. */
function TxMeta({ tx }: { tx: Transaction }) {
  const { t } = useI18n();
  return (
    <>
      {tx.institution} · {tx.account_name}
      {tx.installments > 1 && ` · ${tx.installment}/${tx.installments}`}
      {tx.status === "PENDING" && ` · ${t("fin.tx.pending")}`}
      {tx.split_of && ` · ${t("fin.split.part")}`}
    </>
  );
}

/** A charge that looks billed more than once the same day. */
function Repeated({ n }: { n?: number }) {
  const { t } = useI18n();
  if (!n) return null;
  return <span className="shrink-0 rounded-md bg-amber-500/15 px-1.5 py-0.5 text-xs font-medium text-amber-800 dark:text-amber-300">{fill(t("fin.tx.repeated"), { n })}</span>;
}

/** Clickable filters on desktop; plain text inside the phone row, which is already a button. */
function Tags({ tx, onTag }: { tx: Transaction; onTag?: (tag: string) => void }) {
  const { t } = useI18n();
  if (!tx.tags?.length) return null;
  return (
    <span className="mt-1 flex flex-wrap gap-1">
      {tx.tags.map((tag) => !onTag ? (
        <span key={tag} className="rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground">#{tag}</span>
      ) : (
        <button key={tag} type="button" onClick={(e) => { e.stopPropagation(); onTag(tag); }} title={fill(t("fin.tx.filter_tag"), { t: tag })}
          className="rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground hover:bg-secondary/70">#{tag}</button>
      ))}
    </span>
  );
}

export default function TransactionsPage() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [params, set] = useListParams({ page: 1, limit: 50, q: "", month: monthKey(), account: "", category: "", tag: "" });
  const [search, setSearch] = useState(params.q);
  const [suggestion, setSuggestion] = useState<{ keyword: string; categoryId: string; categoryName: string } | null>(null);
  const [splitting, setSplitting] = useState<Transaction | null>(null);
  const [open, setOpen] = useState<Transaction | null>(null);
  const [fixing, setFixing] = useState<Draft | null>(null);
  const [removing, setRemoving] = useState<Transaction | null>(null);
  // ?new=1 comes from the + of the phone top bar.
  const url = useSearchParams();
  const router = useRouter();
  const [typing, setAdding] = useState(false);
  const adding = typing || url.get("new") === "1";
  const stopAdding = () => {
    setAdding(false);
    if (url.get("new")) router.replace("/transactions");
  };
  const [reviewing, setReviewing] = useState(false);
  const [bulkCategory, setBulkCategory] = useState("");
  const q = searchTerm(useDebounced(params.q));
  const range = monthRange(params.month);
  const filter = { search: q, ...range, account_id: params.account, category_id: params.category, tag: params.tag };

  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories });
  const connections = useQuery({ queryKey: ["finance", "connections"], queryFn: finance.connections });
  const list = useQuery({
    queryKey: ["finance", "transactions", params.page, params.limit, q, params.month, params.account, params.category, params.tag],
    queryFn: ({ signal }) => finance.transactions({ page: params.page, limit: params.limit, ...filter }, signal),
    placeholderData: keepPreviousData,
  });
  // Only the count: the review sheet loads its own rows.
  const toReview = useQuery({
    queryKey: ["finance", "transactions", "uncategorized", params.month],
    queryFn: () => finance.transactions({ page: 1, limit: 1, ...range, category_id: "none" }),
    select: (r) => r.pagination.total_items,
  });

  const suggest = (keyword: string, categoryId: string) => {
    // The server picks the merchant out of the description ("Pagamento de Pix … Uber" gives "uber").
    const cat = categories.data?.items.find((c) => c.id === categoryId);
    setSuggestion(cat && keyword ? { keyword, categoryId, categoryName: cat.name } : null);
  };
  const setCategory = useMutation({
    mutationFn: ({ tx, categoryId }: { tx: Transaction; categoryId: string }) => finance.setCategory(tx.id, categoryId),
    onSuccess: ({ keyword }, { categoryId }) => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      suggest(keyword, categoryId);
      toast.success(t("fin.tx.saved"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const undoToast = useUndoToast();
  const createRule = useMutation({
    mutationFn: (s: { keyword: string; categoryId: string }) => finance.createRule(s.keyword, s.categoryId),
    onSuccess: (r) => {
      setSuggestion(null);
      void client.invalidateQueries({ queryKey: ["finance"] });
      undoToast(fill(t("fin.tx.rule_created"), { n: r.applied }), r.undo_id, r.applied);
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
  const exportCsv = useMutation({
    mutationFn: () => finance.exportTransactions(filter),
    onError: (err) => toast.error(errorMessage(err, "fin.tx.export_failed", t)),
  });

  const label = useAccountLabel();
  const accountItems: Record<string, string> = { "": t("fin.tx.all_accounts") };
  // Accounts with no bank behind them take lines typed by hand.
  const manualItems: Record<string, string> = {};
  for (const c of connections.data?.items ?? []) {
    for (const a of c.accounts) {
      accountItems[a.id] = `${c.institution} · ${label(a)}`;
      if (c.manual) manualItems[a.id] = accountItems[a.id];
    }
  }
  const manual = (tx: Transaction) => !tx.split_of && tx.account_id in manualItems;
  const categoryItems: Record<string, string> = { "": t("fin.tx.all_categories"), [NONE]: t("fin.uncategorized") };
  for (const c of categories.data?.items ?? []) categoryItems[c.id] = c.name;
  const bulkItems: Record<string, string> = { [NONE]: t("fin.uncategorized") };
  for (const c of categories.data?.items ?? []) bulkItems[c.id] = c.name;

  const items = list.data?.items ?? [];
  const dups = repeated(items);
  const selection = useSelection(items.map((tx) => tx.id));
  const bulk = useMutation({
    mutationFn: (categoryId: string) => finance.categorizeMany(selection.ids, categoryId === NONE ? "" : categoryId),
    onSuccess: (r) => {
      selection.clear();
      setBulkCategory("");
      void client.invalidateQueries({ queryKey: ["finance"] });
      undoToast(fill(t("fin.tx.bulk_done"), { n: r.changed }), r.undo_id, r.changed);
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const outflow = items.reduce((n, tx) => n + (tx.amount < 0 && tx.category_kind !== "transfer" ? tx.amount : 0), 0);
  const filtered = q || params.account || params.category || params.tag;
  const filterTag = (tag: string) => set({ tag, page: 1 });

  return (
    <>
      <PageHeader title={t("nav.transactions")} description={t("fin.tx.subtitle")} action={
        <div className="flex flex-wrap items-center gap-2">
          <MonthSwitcher month={params.month} onChange={(m) => set({ month: m, page: 1 })} />
          <Button variant="outline" size="icon" aria-label={t("fin.tx.export")} title={t("fin.tx.export")} disabled={exportCsv.isPending} onClick={() => exportCsv.mutate()}>
            <Download className="size-4" />
          </Button>
          <Button onClick={() => setAdding(true)}><Plus className="size-4" />{t("fin.tx.new")}</Button>
        </div>
      } />
      <PageBody>
        {!!toReview.data && (
          <Card className="flex-row flex-wrap items-center gap-3 p-3 text-sm">
            <ListChecks className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="min-w-0 flex-1">{fill(t("fin.review.banner"), { n: toReview.data })}</span>
            <Button size="sm" onClick={() => setReviewing(true)}>{fill(t("fin.review.open"), { n: toReview.data })}</Button>
          </Card>
        )}
        <div className="flex flex-wrap gap-2 max-sm:grid max-sm:grid-cols-2">
          <div className="relative min-w-56 flex-1 sm:max-w-sm max-sm:col-span-2 max-sm:min-w-0">
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
          {params.tag && (
            <Button variant="secondary" onClick={() => set({ tag: "", page: 1 })} aria-label={`${t("fin.tx.clear_tag")}: #${params.tag}`}>
              #{params.tag}<X className="size-4" />
            </Button>
          )}
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
            action={filtered && <Button variant="outline" onClick={() => { setSearch(""); set({ q: "", account: "", category: "", tag: "", page: 1 }); }}>{t("common.clear_filters")}</Button>} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            {/* Phone and tablet: one compact line per transaction; a tap opens it. */}
            <ul className="divide-y lg:hidden">
              {items.map((tx) => (
                <li key={tx.id}>
                  <button type="button" onClick={() => setOpen(tx)} className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/50">
                    <CategoryDot name={tx.category_name} />
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-1.5"><span className="truncate text-sm font-medium">{merchantName(tx.description)}</span><Repeated n={dups.get(tx.id)} /></span>
                      <span className={cn("block truncate text-xs", tx.category_id ? "text-muted-foreground" : "text-amber-700 dark:text-amber-400")}>
                        {tx.category_name || t("fin.uncategorized")} · {f.day(tx.date)}{tx.installments > 1 && ` · ${tx.installment}/${tx.installments}`}
                      </span>
                      <Tags tx={tx} />
                    </span>
                    <span className={cn("shrink-0 font-mono text-sm tabular-nums", amountClass(tx.amount))}>{f.money(tx.amount)}</span>
                  </button>
                </li>
              ))}
            </ul>
            {/* Desktop: the table, with the category one click away and rows to tick for bulk changes. */}
            <Table className="max-lg:hidden" data-compact>
              <TableHeader>
                <TableRow>
                  <SelectAllHead selection={selection} />
                  <TableHead className="w-20">{t("fin.tx.date")}</TableHead>
                  <TableHead>{t("fin.tx.description")}</TableHead>
                  <TableHead className="w-52">{t("fin.tx.category")}</TableHead>
                  <TableHead className="w-36 text-right">{t("fin.tx.amount")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((tx) => (
                  <TableRow key={tx.id} className="cursor-pointer" onClick={() => setOpen(tx)} data-state={selection.has(tx.id) ? "selected" : undefined}>
                    <SelectCell selection={selection} id={tx.id} label={`${t("fin.tx.select")}: ${tx.description}`} />
                    <TableCell className="text-muted-foreground tabular-nums">{f.day(tx.date)}</TableCell>
                    <TableCell className="w-full max-w-0">
                      <div className="flex items-center gap-3">
                        <CategoryDot name={tx.category_name} />
                        <div className="min-w-0">
                          <p className="flex items-center gap-1.5"><span className="truncate font-medium" title={tx.description}>{merchantName(tx.description)}</span><Repeated n={dups.get(tx.id)} /></p>
                          <p className="truncate text-xs text-muted-foreground">{tx.description} · <TxMeta tx={tx} /></p>
                          {tx.note && <p className="truncate text-xs italic text-muted-foreground" title={tx.note}>{tx.note}</p>}
                          <Tags tx={tx} onTag={filterTag} />
                        </div>
                      </div>
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <CategoryPicker tx={tx} categories={categories.data?.items ?? []} onPick={(tx, categoryId) => setCategory.mutate({ tx, categoryId })} />
                    </TableCell>
                    <TableCell className={cn("text-right font-mono tabular-nums", amountClass(tx.amount))}>{f.money(tx.amount)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="border-t px-4 py-2 text-right text-sm text-muted-foreground">{fill(t("fin.tx.outflow"), { v: f.money(-outflow) })}</div>
            <TablePagination page={params.page} pageSize={params.limit} totalItems={list.data.pagination.total_items}
              onPageChange={(n) => set({ page: n })} onPageSizeChange={(n) => set({ limit: n, page: 1 })} />
          </Card>
        )}
        <SelectionBar selection={selection}>
          <Select value={bulkCategory} items={bulkItems} onValueChange={(v) => { const id = String(v ?? ""); setBulkCategory(id); if (id) bulk.mutate(id); }}>
            <SelectTrigger className="w-56" aria-label={t("fin.tx.bulk_category")} disabled={bulk.isPending}><SelectValue placeholder={t("fin.tx.bulk_category")} /></SelectTrigger>
            <SelectContent>{Object.entries(bulkItems).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
          </Select>
        </SelectionBar>
        <SplitDialog tx={splitting} categories={categories.data?.items ?? []} onClose={() => setSplitting(null)} />
        {open && (
          <TransactionSheet key={open.id} tx={open} categories={categories.data?.items ?? []} manual={manual(open)} repeated={dups.get(open.id)} onClose={() => setOpen(null)}
            onCategorized={suggest}
            onSplit={() => { setSplitting(open); setOpen(null); }}
            onUnsplit={() => { unsplit.mutate(open.split_of); setOpen(null); }}
            onRemove={() => { setRemoving(open); setOpen(null); }}
            onFixed={() => {
              const name = merchantName(open.description);
              setFixing({ name, amount: -open.amount, day: Number(open.date.slice(8, 10)), month: 0, keyword: name.toLowerCase(), category_id: open.category_id ?? "", account_id: open.account_id });
              setOpen(null);
            }} />
        )}
        {fixing && <SubscriptionDialog draft={fixing} onClose={() => setFixing(null)} />}
        {adding && <NewTransactionSheet accounts={manualItems} categories={categories.data?.items ?? []} onClose={stopAdding} />}
        {reviewing && <ReviewSheet range={range} categories={categories.data?.items ?? []} onClose={() => setReviewing(false)} />}
        <ConfirmDialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)} destructive
          title={fill(t("fin.tx.remove_title"), { d: removing?.description ?? "" })} description={t("fin.tx.remove_help")} confirmLabel={t("fin.tx.remove")}
          onConfirm={async () => {
            await finance.removeTransaction(removing!.id);
            void client.invalidateQueries({ queryKey: ["finance"] });
            toast.success(t("fin.tx.removed"));
          }} />
      </PageBody>
    </>
  );
}
