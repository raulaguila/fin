"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { IconAction, PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance, monthKey, type Category, type CategoryKind, type RuleSuggestion } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { useDebounced } from "@/lib/hooks/use-debounced";
import { useI18n } from "@/lib/i18n/provider";
import type { MessageKey } from "@/lib/i18n/messages";

const KINDS: CategoryKind[] = ["expense", "income", "transfer"];

function useInvalidate() {
  const client = useQueryClient();
  return () => client.invalidateQueries({ queryKey: ["finance"] });
}

function Rules({ categories }: { categories: Category[] }) {
  const { t } = useI18n();
  const invalidate = useInvalidate();
  const [keyword, setKeyword] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const typed = useDebounced(keyword.trim());
  const rules = useQuery({ queryKey: ["finance", "rules"], queryFn: finance.rules });
  const preview = useQuery({ queryKey: ["finance", "rules", "preview", typed], queryFn: () => finance.previewRule(typed), enabled: typed.length >= 2 });
  const create = useMutation({
    mutationFn: () => finance.createRule(keyword.trim(), categoryId),
    onSuccess: (r) => {
      setKeyword("");
      void invalidate();
      toast.success(fill(t("fin.tx.rule_created"), { n: r.applied }));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const remove = useMutation({
    mutationFn: finance.removeRule,
    onSuccess: () => void invalidate(),
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const items: Record<string, string> = Object.fromEntries(categories.map((c) => [c.id, c.name]));

  return (
    <Card className="gap-3 p-4">
      <h2 className="text-sm font-medium">{t("fin.cat.rules")}</h2>
      <form className="flex flex-wrap items-end gap-2" onSubmit={(e) => { e.preventDefault(); if (keyword.trim().length >= 2 && categoryId) create.mutate(); }}>
        <label className="grid min-w-48 flex-1 gap-1 text-xs text-muted-foreground">
          {t("fin.cat.keyword")}
          <Input value={keyword} maxLength={60} onChange={(e) => setKeyword(e.target.value)} placeholder="uber, ifood, netflix…" />
        </label>
        <Select value={categoryId} items={items} onValueChange={(v) => setCategoryId(String(v ?? ""))}>
          <SelectTrigger className="w-56 max-sm:w-full" aria-label={t("fin.tx.category")}><SelectValue placeholder={t("fin.tx.category")} /></SelectTrigger>
          <SelectContent>{categories.map((c) => <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>)}</SelectContent>
        </Select>
        <Button type="submit" disabled={keyword.trim().length < 2 || !categoryId || create.isPending}><Plus className="size-4" />{t("fin.cat.add")}</Button>
      </form>
      {preview.data && typed.length >= 2 && <p className="text-xs text-muted-foreground" role="status">{fill(t("fin.cat.preview"), { n: preview.data.matches })}</p>}
      {rules.data?.items.length === 0 && <p className="text-sm text-muted-foreground">{t("fin.cat.no_rules")}</p>}
      <ul className="divide-y">
        {rules.data?.items.map((r) => (
          <li key={r.id} className="flex items-center justify-between gap-3 py-2 text-sm">
            <span className="min-w-0 truncate">
              <span className="text-muted-foreground">{t("fin.cat.keyword")}</span> <strong>“{r.keyword}”</strong> → {r.category_name}
              <span className="text-xs text-muted-foreground"> · {fill(t("fin.cat.hits"), { n: r.hits })}</span>
            </span>
            <IconAction label={t("fin.cat.remove")} target={r.keyword} onClick={() => remove.mutate(r.id)}><Trash2 className="size-4" /></IconAction>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function Suggestion({ s, categories }: { s: RuleSuggestion; categories: Category[] }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const invalidate = useInvalidate();
  const [categoryId, setCategoryId] = useState("");
  const items: Record<string, string> = Object.fromEntries(categories.map((c) => [c.id, c.name]));
  const create = useMutation({
    mutationFn: () => finance.createRule(s.keyword, categoryId),
    onSuccess: (r) => {
      void invalidate();
      toast.success(fill(t("fin.tx.rule_created"), { n: r.applied }));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <li className="flex flex-wrap items-center justify-between gap-2 py-2 text-sm">
      <span className="min-w-0 flex-1">
        <strong>“{s.keyword}”</strong>
        <span className="block truncate text-xs text-muted-foreground" title={s.example}>
          {fill(t("fin.cat.suggestion_line"), { n: s.count, v: f.money(Math.abs(s.total)) })} · {s.example}
        </span>
      </span>
      <span className="flex items-center gap-2 max-sm:w-full">
        <Select value={categoryId} items={items} onValueChange={(v) => setCategoryId(String(v ?? ""))}>
          <SelectTrigger className="w-44 max-sm:flex-1" aria-label={`${t("fin.tx.category")}: ${s.keyword}`}><SelectValue placeholder={t("fin.tx.category")} /></SelectTrigger>
          <SelectContent>{categories.map((c) => <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>)}</SelectContent>
        </Select>
        <Button size="sm" disabled={!categoryId || create.isPending} onClick={() => create.mutate()}>{t("fin.cat.create_rule")}</Button>
      </span>
    </li>
  );
}

function Suggestions({ categories }: { categories: Category[] }) {
  const { t } = useI18n();
  const list = useQuery({ queryKey: ["finance", "rules", "suggestions"], queryFn: finance.ruleSuggestions });
  if (!list.data?.items.length) return null;
  return (
    <Card className="gap-3 p-4">
      <div>
        <h2 className="text-sm font-medium">{t("fin.cat.suggestions")}</h2>
        <p className="text-xs text-muted-foreground">{t("fin.cat.suggestions_note")}</p>
      </div>
      <ul className="divide-y">{list.data.items.map((s) => <Suggestion key={s.keyword} s={s} categories={categories} />)}</ul>
    </Card>
  );
}

function NewCategory() {
  const { t } = useI18n();
  const invalidate = useInvalidate();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<CategoryKind>("expense");
  const kinds = Object.fromEntries(KINDS.map((k) => [k, t(`fin.kind.${k}` as MessageKey)]));
  const create = useMutation({
    mutationFn: () => finance.createCategory({ name: name.trim(), kind, limit: 0 }),
    onSuccess: () => {
      setName("");
      void invalidate();
      toast.success(t("fin.cat.created"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <form className="flex flex-wrap items-end gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate(); }}>
      <label className="grid min-w-48 flex-1 gap-1 text-xs text-muted-foreground">
        {t("fin.cat.new")}
        <Input value={name} maxLength={40} onChange={(e) => setName(e.target.value)} placeholder={t("fin.cat.name")} />
      </label>
      <Select value={kind} items={kinds} onValueChange={(v) => setKind(v as CategoryKind)}>
        <SelectTrigger className="w-44 max-sm:w-full" aria-label={t("fin.cat.kind")}><SelectValue /></SelectTrigger>
        <SelectContent>{KINDS.map((k) => <SelectItem key={k} value={k}>{kinds[k]}</SelectItem>)}</SelectContent>
      </Select>
      <Button type="submit" variant="outline" disabled={!name.trim() || create.isPending}><Plus className="size-4" />{t("fin.cat.add")}</Button>
    </form>
  );
}

function CategoryRow({ c, total, onDelete }: { c: Category; total?: number; onDelete: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const invalidate = useInvalidate();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(c.name);
  const rename = useMutation({
    mutationFn: () => finance.updateCategory(c.id, { name: name.trim() }),
    onSuccess: () => { setEditing(false); void invalidate(); },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <li className="flex items-center justify-between gap-3 py-2 text-sm">
      {editing ? (
        <form className="flex flex-1 gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim() && name.trim() !== c.name) rename.mutate(); else setEditing(false); }}>
          <Input autoFocus value={name} maxLength={40} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === "Escape" && setEditing(false)} aria-label={t("fin.cat.name")} />
          <Button type="submit" size="sm" disabled={rename.isPending}>{t("common.save")}</Button>
        </form>
      ) : (
        <span className="min-w-0 truncate">{c.name}</span>
      )}
      <span className="flex shrink-0 items-center gap-1">
        {total !== undefined && total !== 0 && <span className="mr-2 font-mono text-xs tabular-nums text-muted-foreground">{f.money(Math.abs(total))}</span>}
        {!editing && <IconAction label={t("fin.cat.rename")} target={c.name} onClick={() => setEditing(true)}><Pencil className="size-4" /></IconAction>}
        <IconAction label={t("common.delete")} target={c.name} onClick={onDelete}><Trash2 className="size-4" /></IconAction>
      </span>
    </li>
  );
}

export default function CategoriesPage() {
  const { t } = useI18n();
  const invalidate = useInvalidate();
  const categories = useQuery({ queryKey: ["finance", "categories"], queryFn: finance.categories });
  const month = monthKey();
  const summary = useQuery({ queryKey: ["finance", "summary", month], queryFn: () => finance.summary(month) });
  const [deleting, setDeleting] = useState<Category | null>(null);
  const recategorize = useMutation({
    mutationFn: finance.recategorize,
    onSuccess: (r) => {
      void invalidate();
      toast.success(fill(t("fin.cat.recategorized"), { n: r.changed }));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const totals = new Map(summary.data?.categories.map((c) => [c.category_id, c.total]));
  const list = categories.data?.items ?? [];

  return (
    <>
      <PageHeader title={t("nav.categories")} description={t("fin.cat.subtitle")}
        action={<Button variant="outline" disabled={recategorize.isPending} onClick={() => recategorize.mutate()}><RefreshCw className="size-4" />{t("fin.cat.recategorize")}</Button>} />
      <PageBody>
        {categories.isPending ? (
          <ListSkeleton />
        ) : categories.isError ? (
          <QueryErrorState error={categories.error} onRetry={() => void categories.refetch()} />
        ) : (
          <div className="grid gap-4 lg:grid-cols-2 lg:items-start">
            <div className="grid gap-4">
              <Rules categories={list} />
              <Suggestions categories={list} />
            </div>
            <Card className="gap-3 p-4">
              <div className="flex items-baseline justify-between gap-2">
                <h2 className="text-sm font-medium">{t("fin.cat.categories")}</h2>
                <span className="text-xs text-muted-foreground">{t("fin.cat.spent_in")}</span>
              </div>
              <NewCategory />
              {KINDS.map((kind) => (
                <section key={kind} className="space-y-1">
                  <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t(`fin.kind.${kind}` as MessageKey)}</h3>
                  <ul className="divide-y">
                    {list.filter((c) => c.kind === kind).map((c) => <CategoryRow key={c.id} c={c} total={totals.get(c.id)} onDelete={() => setDeleting(c)} />)}
                  </ul>
                </section>
              ))}
              <p className="text-xs text-muted-foreground">{t("fin.cat.transfer_note")}</p>
            </Card>
          </div>
        )}
        <ConfirmDialog
          open={!!deleting}
          onOpenChange={(o) => !o && setDeleting(null)}
          title={fill(t("fin.cat.delete_title"), { c: deleting?.name ?? "" })}
          description={t("fin.cat.delete_body")}
          destructive
          confirmLabel={t("common.delete")}
          onConfirm={async () => {
            if (!deleting) return;
            await finance.removeCategory(deleting.id);
            setDeleting(null);
            await invalidate();
          }}
        />
      </PageBody>
    </>
  );
}
