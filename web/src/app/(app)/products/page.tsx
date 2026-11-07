"use client";

import { useState } from "react";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, Eye, Package, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { ActiveBadge, IconAction, PageBody, PageHeader, useFormatDate } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { SelectAllHead, SelectCell, SelectionBar, useSelection } from "@/components/selection";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination } from "@/components/table-pagination";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { products } from "@/lib/api/endpoints";
import type { Product } from "@/lib/api/types";
import { CURRENCY } from "@/lib/brand";
import { useMe } from "@/lib/context/me";
import { useDebounced } from "@/lib/hooks/use-debounced";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { can } from "@/lib/permissions";

type Sort = "name" | "price" | "created_at";
type State = "all" | "on" | "off";

export default function ProductsPage() {
  const { t, locale } = useI18n();
  const format = useFormatDate();
  const { user } = useMe();
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<Sort>("created_at");
  const [order, setOrder] = useState<"asc" | "desc">("desc");
  const [state, setState] = useState<State>("all");
  const router = useRouter();
  const [deleting, setDeleting] = useState<Product | null>(null);
  const [deletingMany, setDeletingMany] = useState(false);
  const q = useDebounced(search.trim());
  const list = useQuery({
    queryKey: ["products", page, limit, q, sort, order, state],
    queryFn: () => products.list({ page, limit, search: q, sort, order, enabled: state === "all" ? undefined : state === "on" }),
    placeholderData: keepPreviousData,
  });
  const canEdit = can(user, "products", "update");
  const selection = useSelection(list.data?.items.map((p) => p.id) ?? []);
  const open = (id: string) => router.push(`/products/${id}`);
  const canDelete = can(user, "products", "delete");
  const money = new Intl.NumberFormat(locale, { style: "currency", currency: CURRENCY });
  const stateItems: Record<State, string> = { all: t("products.all_states"), on: t("roles.enabled"), off: t("roles.disabled") };

  const sortBy = (field: Sort) => {
    if (sort === field) setOrder((o) => (o === "asc" ? "desc" : "asc"));
    else { setSort(field); setOrder(field === "created_at" ? "desc" : "asc"); }
    setPage(1);
  };
  const sortHead = (field: Sort, label: MessageKey, className?: string) => (
    <TableHead key={field} className={className} aria-sort={sort === field ? (order === "asc" ? "ascending" : "descending") : undefined}>
      <button type="button" className="inline-flex items-center gap-1 hover:text-foreground" onClick={() => sortBy(field)}>
        {t(label)}
        {sort === field && (order === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />)}
      </button>
    </TableHead>
  );

  return (
    <>
      <PageHeader
        title={t("nav.products")}
        description={t("products.subtitle")}
        action={can(user, "products", "create") && <Button onClick={() => open("new")}><Plus className="size-4" />{t("products.new")}</Button>}
      />
      <PageBody>
        <div className="flex flex-wrap gap-2">
          <div className="relative min-w-56 flex-1 sm:max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input type="search" aria-label={t("common.search")} placeholder={t("products.search")} className="pl-9" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
          </div>
          <Select value={state} items={stateItems} onValueChange={(v) => { setState(v as State); setPage(1); }}>
            <SelectTrigger className="w-44" aria-label={t("roles.state")}><SelectValue /></SelectTrigger>
            <SelectContent>
              {Object.entries(stateItems).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>

        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState onRetry={() => void list.refetch()} />
        ) : list.data.items.length === 0 ? (
          <EmptyState icon={Package} title={t("products.empty")} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            <Table data-responsive>
              <TableHeader>
                <TableRow>
                  {canDelete && <SelectAllHead selection={selection} />}
                  {sortHead("name", "auth.name")}
                  {sortHead("price", "products.price", "w-36 text-right")}
                  <TableHead className="w-28">{t("roles.state")}</TableHead>
                  {sortHead("created_at", "common.created_at", "w-44")}
                  <TableHead className="w-24"><span className="sr-only">{t("common.actions")}</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.data.items.map((p) => (
                  <TableRow key={p.id} className="cursor-pointer" onClick={() => open(p.id)}>
                    {canDelete && <SelectCell selection={selection} id={p.id} label={p.name} />}
                    <TableCell data-label={t("auth.name")} className="font-medium">{p.name}</TableCell>
                    <TableCell data-label={t("products.price")} className="text-right font-mono tabular-nums">{money.format(Number(p.price))}</TableCell>
                    <TableCell data-label={t("roles.state")}>
                      <ActiveBadge enabled={p.enabled} />
                    </TableCell>
                    <TableCell data-label={t("common.created_at")} className="text-muted-foreground">{format(p.created_at)}</TableCell>
                    <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                      <IconAction label={t(canEdit ? "common.edit" : "common.view")} target={p.name} onClick={() => open(p.id)}>
                        {canEdit ? <Pencil className="size-4" /> : <Eye className="size-4" />}
                      </IconAction>
                      {canDelete && <IconAction label={t("common.delete")} target={p.name} className="text-destructive hover:bg-destructive/10 hover:text-destructive" onClick={() => setDeleting(p)}><Trash2 className="size-4" /></IconAction>}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <TablePagination
              page={page}
              pageSize={limit}
              totalItems={list.data.pagination.total_items}
              onPageChange={setPage}
              onPageSizeChange={(n) => { setLimit(n); setPage(1); }}
            />
          </Card>
        )}
        <SelectionBar selection={selection}>
          <Button variant="destructive" onClick={() => setDeletingMany(true)}><Trash2 className="size-4" />{t("common.delete_selected")}</Button>
        </SelectionBar>
      </PageBody>
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={t("products.delete")}
        description={deleting?.name}
        destructive
        confirmLabel={t("common.delete")}
        onConfirm={async () => {
          await products.remove(deleting!.id);
          await client.invalidateQueries({ queryKey: ["products"] });
        }}
      />
      <ConfirmDialog
        open={deletingMany}
        onOpenChange={setDeletingMany}
        title={t("common.delete_selected")}
        description={t("common.delete_selected_desc")}
        destructive
        confirmLabel={t("common.delete")}
        onConfirm={async () => {
          await products.removeMany(selection.ids);
          selection.clear();
          await client.invalidateQueries({ queryKey: ["products"] });
        }}
      />
    </>
  );
}
