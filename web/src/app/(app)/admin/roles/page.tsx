"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, KeyRound, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { ActiveBadge, IconAction, PageBody, PageHeader, useFormatDate } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { SelectAllHead, SelectCell, SelectionBar, useSelection } from "@/components/selection";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination } from "@/components/table-pagination";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { roles } from "@/lib/api/endpoints";
import type { Action, Role } from "@/lib/api/types";
import { useMe } from "@/lib/context/me";
import { useDebounced } from "@/lib/hooks/use-debounced";
import { useI18n } from "@/lib/i18n/provider";
import type { MessageKey } from "@/lib/i18n/messages";
import { can, resourceText } from "@/lib/permissions";

/** "Produtos: ler, editar" chips, three at most. */
function PermissionChips({ role }: { role: Role }) {
  const { t } = useI18n();
  if ("*" in role.permissions) return <Badge variant="secondary">{t("roles.all_permissions")}</Badge>;
  const entries = Object.entries(role.permissions).filter(([, a]) => a.length > 0);
  if (entries.length === 0) return <span className="text-muted-foreground">-</span>;
  const chip = ([res, actions]: [string, Action[]]) =>
    `${resourceText(t, res).label}: ${actions.map((a) => t(`action.${a}` as MessageKey).toLowerCase()).join(", ")}`;
  return (
    <div className="flex flex-wrap gap-1.5">
      {entries.slice(0, 3).map((e) => <Badge key={e[0]} variant="outline" className="font-normal">{chip(e)}</Badge>)}
      {entries.length > 3 && <Badge variant="outline" title={entries.slice(3).map(chip).join("\n")}>+{entries.length - 3}</Badge>}
    </div>
  );
}

export default function RolesPage() {
  const { t } = useI18n();
  const format = useFormatDate();
  const { user } = useMe();
  const router = useRouter();
  const client = useQueryClient();
  const canEdit = can(user, "roles", "update");
  const canDelete = can(user, "roles", "delete");
  const [deleting, setDeleting] = useState(false);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [search, setSearch] = useState("");
  const q = useDebounced(search.trim());
  const list = useQuery({
    queryKey: ["roles", "list", page, limit, q],
    queryFn: () => roles.list({ page, limit, search: q }),
    placeholderData: keepPreviousData,
  });
  // A default role cannot be deleted, so it is never ticked.
  const selection = useSelection(list.data?.items.filter((r) => !r.is_default).map((r) => r.id) ?? []);
  const open = (id: string) => router.push(`/admin/roles/${id}`);

  return (
    <>
      <PageHeader
        title={t("nav.roles")}
        description={t("roles.subtitle")}
        action={can(user, "roles", "create") && <Button onClick={() => router.push("/admin/roles/new")}><Plus className="size-4" />{t("roles.new")}</Button>}
      />
      <PageBody>
        <div className="relative max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input type="search" aria-label={t("common.search")} placeholder={t("roles.search")} className="pl-9" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
        </div>
        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState onRetry={() => void list.refetch()} />
        ) : list.data.items.length === 0 ? (
          <EmptyState icon={KeyRound} title={t("roles.empty")} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            <Table data-responsive>
              <TableHeader>
                <TableRow>
                  {canDelete && <SelectAllHead selection={selection} />}
                  <TableHead>{t("auth.name")}</TableHead>
                  <TableHead>{t("roles.permissions")}</TableHead>
                  <TableHead className="w-24 text-right">{t("roles.members")}</TableHead>
                  <TableHead className="w-28">{t("roles.state")}</TableHead>
                  <TableHead className="w-44">{t("common.created_at")}</TableHead>
                  <TableHead className="w-14"><span className="sr-only">{t("common.actions")}</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.data.items.map((r) => (
                  <TableRow key={r.id} className="cursor-pointer" onClick={() => open(r.id)}>
                    {canDelete && <SelectCell selection={selection} id={r.id} label={r.name} disabled={r.is_default} />}
                    <TableCell data-label={t("auth.name")}>
                      <span className="font-medium">{r.name}</span>
                      {r.is_default && <Badge variant="outline" className="ml-2">{t("roles.default")}</Badge>}
                    </TableCell>
                    <TableCell data-label={t("roles.permissions")}><PermissionChips role={r} /></TableCell>
                    <TableCell data-label={t("roles.members")} className="text-right tabular-nums">{r.member_count}</TableCell>
                    <TableCell data-label={t("roles.state")}><ActiveBadge enabled={r.enabled} /></TableCell>
                    <TableCell data-label={t("common.created_at")} className="text-muted-foreground">{format(r.created_at)}</TableCell>
                    <TableCell className="text-right">
                      <IconAction label={t(canEdit ? "common.edit" : "common.view")} target={r.name} onClick={() => open(r.id)}>
                        {canEdit ? <Pencil className="size-4" /> : <Eye className="size-4" />}
                      </IconAction>
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
        <p className="text-xs text-muted-foreground">{t("roles.protected_note")}</p>
        <SelectionBar selection={selection}>
          <Button variant="destructive" onClick={() => setDeleting(true)}><Trash2 className="size-4" />{t("common.delete_selected")}</Button>
        </SelectionBar>
      </PageBody>
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t("common.delete_selected")}
        description={t("common.delete_selected_desc")}
        destructive
        confirmLabel={t("common.delete")}
        onConfirm={async () => {
          await roles.removeMany(selection.ids);
          selection.clear();
          await client.invalidateQueries({ queryKey: ["roles"] });
        }}
      />
    </>
  );
}
