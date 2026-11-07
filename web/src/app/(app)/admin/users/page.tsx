"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, Pencil, Search, UserCheck, UserX, Users } from "lucide-react";
import { toast } from "sonner";
import { IconAction, PageBody, PageHeader, StatusBadge, useFormatDate } from "@/components/bits";
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
import { users } from "@/lib/api/endpoints";
import type { UserStatus } from "@/lib/api/types";
import { useMe } from "@/lib/context/me";
import { useDebounced } from "@/lib/hooks/use-debounced";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { can } from "@/lib/permissions";

const STATUSES: UserStatus[] = ["active", "pending", "locked", "disabled"];

export default function UsersPage() {
  const { t } = useI18n();
  const format = useFormatDate();
  const router = useRouter();
  const { user } = useMe();
  // Editing a user means changing state or roles.
  const canEdit = can(user, "users", "update") || can(user, "roles", "update");
  const canBulk = can(user, "users", "update");
  const client = useQueryClient();
  const [bulk, setBulk] = useState<"disable" | "enable" | null>(null);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<UserStatus | "">("");
  const q = useDebounced(search.trim());
  const list = useQuery({
    queryKey: ["users", page, limit, q, status],
    queryFn: () => users.list({ page, limit, search: q, status }),
    placeholderData: keepPreviousData,
  });
  // Your own account cannot be disabled, so it is never ticked.
  const selection = useSelection(list.data?.items.filter((u) => u.id !== user?.id).map((u) => u.id) ?? []);
  const statusItems: Record<string, string> = { all: t("users.all_statuses"), ...Object.fromEntries(STATUSES.map((s) => [s, t(`status.${s}` as MessageKey)])) };

  return (
    <>
      <PageHeader title={t("nav.users")} description={t("users.subtitle")} />
      <PageBody>
        <div className="flex flex-wrap gap-2">
          <div className="relative min-w-56 flex-1 sm:max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label={t("common.search")}
              placeholder={t("users.search")}
              className="pl-9"
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            />
          </div>
          <Select value={status || "all"} items={statusItems} onValueChange={(v) => { setStatus(v === "all" ? "" : (v as UserStatus)); setPage(1); }}>
            <SelectTrigger className="w-44" aria-label={t("users.status")}><SelectValue /></SelectTrigger>
            <SelectContent>
              {Object.entries(statusItems).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>

        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState onRetry={() => void list.refetch()} />
        ) : list.data.items.length === 0 ? (
          <EmptyState icon={Users} title={t("users.empty")} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            <Table data-responsive>
              <TableHeader>
                <TableRow>
                  {canBulk && <SelectAllHead selection={selection} />}
                  <TableHead>{t("auth.name")}</TableHead>
                  <TableHead>{t("auth.email")}</TableHead>
                  <TableHead className="w-32">{t("users.status")}</TableHead>
                  <TableHead className="w-44">{t("common.created_at")}</TableHead>
                  <TableHead className="w-14"><span className="sr-only">{t("common.actions")}</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.data.items.map((u) => (
                  <TableRow key={u.id} className="cursor-pointer" onClick={() => router.push(`/admin/users/${u.id}`)}>
                    {canBulk && <SelectCell selection={selection} id={u.id} label={u.name} disabled={u.id === user?.id} />}
                    <TableCell data-label={t("auth.name")}>
                      <span className="font-medium">{u.name}</span>
                    </TableCell>
                    <TableCell data-label={t("auth.email")}>{u.email}</TableCell>
                    <TableCell data-label={t("users.status")}><StatusBadge status={u.status} /></TableCell>
                    <TableCell data-label={t("common.created_at")} className="text-muted-foreground">{format(u.created_at)}</TableCell>
                    <TableCell className="text-right">
                      <IconAction label={t(canEdit ? "common.edit" : "common.view")} target={u.name} onClick={() => router.push(`/admin/users/${u.id}`)}>
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
        <SelectionBar selection={selection}>
          <Button variant="outline" onClick={() => setBulk("enable")}><UserCheck className="size-4" />{t("users.enable_selected")}</Button>
          <Button variant="destructive" onClick={() => setBulk("disable")}><UserX className="size-4" />{t("users.disable_selected")}</Button>
        </SelectionBar>
      </PageBody>
      <ConfirmDialog
        open={bulk !== null}
        onOpenChange={(o) => !o && setBulk(null)}
        title={t(bulk === "enable" ? "users.enable_selected" : "users.disable_selected")}
        description={t(bulk === "enable" ? "users.enable_selected_desc" : "users.disable_selected_desc")}
        destructive={bulk === "disable"}
        onConfirm={async () => {
          await (bulk === "enable" ? users.enableMany : users.disableMany)(selection.ids);
          selection.clear();
          toast.success(t("common.saved"));
          await client.invalidateQueries({ queryKey: ["users"] });
        }}
      />
    </>
  );
}
