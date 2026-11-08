"use client";

import { useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Plus, Search, X } from "lucide-react";
import { toast } from "sonner";
import { FormError, PageBody, PageHeader, StatusBadge, useFormatDate } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination, emptyPageFallback } from "@/components/table-pagination";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api/client";
import { roles, users } from "@/lib/api/endpoints";
import type { Action, Role } from "@/lib/api/types";
import { useMe } from "@/lib/context/me";
import { errorMessage } from "@/lib/errors";
import { useDebounced } from "@/lib/hooks/use-debounced";
import { useUnsavedGuard } from "@/lib/hooks/use-unsaved-guard";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { can, resourceText } from "@/lib/permissions";
import { useCrumb } from "@/lib/store/crumb";

const ACTIONS: Action[] = ["create", "read", "update", "delete"];
type Perms = Record<string, Action[]>;

/** Drops empty resources and orders actions, so two equal sets compare equal. */
function clean(p: Perms): Perms {
  return Object.fromEntries(
    Object.entries(p)
      .map(([r, a]) => [r, ACTIONS.filter((x) => a.includes(x))] as const)
      .filter(([, a]) => a.length > 0)
      .sort(([a], [b]) => a.localeCompare(b)),
  );
}

/** A check box that is on, off or partly on, for select-all controls. */
function AllBox({ label, on, total, disabled, onChange }: { label: string; on: number; total: number; disabled: boolean; onChange: (on: boolean) => void }) {
  return (
    <Checkbox
      aria-label={label}
      disabled={disabled}
      checked={total > 0 && on === total}
      indeterminate={on > 0 && on < total}
      onCheckedChange={() => onChange(on < total)}
    />
  );
}

function GeneralCard({ name, enabled, isDefault, editable, onName, onEnabled, onDefault }: {
  name: string;
  enabled: boolean;
  isDefault: boolean;
  editable: boolean;
  onName: (v: string) => void;
  onEnabled: (v: boolean) => void;
  onDefault: (v: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <Card>
      <CardHeader><CardTitle>{t("roles.general")}</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="name">{t("auth.name")}</Label>
          <Input id="name" required maxLength={40} disabled={!editable} value={name} onChange={(e) => onName(e.target.value)} />
        </div>
        <label className="flex items-center gap-3 text-sm">
          {/* The default role must stay enabled. */}
          <Switch checked={enabled} disabled={!editable || isDefault} onCheckedChange={onEnabled} aria-label={t("roles.enabled")} />
          {t(enabled ? "roles.enabled" : "roles.disabled")}
        </label>
        <p className="text-xs text-muted-foreground">{t("roles.enabled_hint")}</p>
        <label className="flex items-center gap-3 text-sm">
          <Switch checked={isDefault} disabled={!editable} onCheckedChange={onDefault} aria-label={t("roles.make_default")} />
          {t("roles.make_default")}
        </label>
        <p className="text-xs text-muted-foreground">{t(isDefault ? "roles.default_note" : "roles.make_default_hint")}</p>
      </CardContent>
    </Card>
  );
}

function PermissionsCard({ all, draft, editable, onChange }: { all: boolean; draft: Perms; editable: boolean; onChange: (p: Perms) => void }) {
  const { t } = useI18n();
  const resources = useQuery({ queryKey: ["resources"], queryFn: roles.resources, staleTime: Infinity });
  const codes = resources.data?.items.map((r) => r.code) ?? [];
  const has = (res: string, a: Action) => draft[res]?.includes(a) ?? false;
  const set = (pairs: [string, Action][], on: boolean) => {
    const next = { ...draft };
    for (const [res, a] of pairs) {
      const cur = next[res] ?? [];
      next[res] = on ? (cur.includes(a) ? cur : [...cur, a]) : cur.filter((x) => x !== a);
    }
    onChange(next);
  };
  const cells = (rs: string[], as: Action[]) => rs.flatMap((r) => as.map((a) => [r, a] as [string, Action]));
  const count = (pairs: [string, Action][]) => pairs.filter(([r, a]) => has(r, a)).length;
  const actionLabel = (a: Action) => t(`action.${a}` as MessageKey);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("roles.permissions")}</CardTitle>
        <CardDescription>{t(all ? "roles.all_permissions_desc" : "roles.permissions_desc")}</CardDescription>
      </CardHeader>
      {!all && (
        <CardContent>
          {resources.isPending ? (
            <ListSkeleton rows={3} />
          ) : resources.isError ? (
            <QueryErrorState onRetry={() => void resources.refetch()} />
          ) : (
            <div className="overflow-x-auto rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("roles.resource")}</TableHead>
                    {ACTIONS.map((a) => {
                      const col = cells(codes, [a]);
                      return (
                        <TableHead key={a} className="w-16 text-center px-1 [&:has([role=checkbox])]:px-1">
                          <div className="py-1">{actionLabel(a)}</div>
                          <div className="flex justify-center pb-1">
                            <AllBox label={`${t("common.select_all")}: ${actionLabel(a)}`} on={count(col)} total={col.length} disabled={!editable} onChange={(on) => set(col, on)} />
                          </div>
                        </TableHead>
                      );
                    })}
                    <TableHead className="w-16 border-l bg-muted/40 text-center px-1 [&:has([role=checkbox])]:px-1">
                      <div className="py-1">{t("roles.all_permissions")}</div>
                      <div className="flex justify-center pb-1">
                        <AllBox label={t("common.select_all")} on={count(cells(codes, ACTIONS))} total={codes.length * ACTIONS.length} disabled={!editable} onChange={(on) => set(cells(codes, ACTIONS), on)} />
                      </div>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {resources.data.items.map((r) => {
                    const text = resourceText(t, r.code, r.description);
                    const row = cells([r.code], ACTIONS);
                    return (
                      <TableRow key={r.code}>
                        <TableCell>
                          <button
                            type="button"
                            disabled={!editable}
                            title={editable ? t("common.select_all") : undefined}
                            onClick={() => set(row, count(row) < row.length)}
                            className="text-left enabled:cursor-pointer enabled:hover:text-primary"
                          >
                            <span className="block font-medium">{text.label}</span>
                            <span className="block text-xs text-muted-foreground">{text.description}</span>
                          </button>
                        </TableCell>
                        {ACTIONS.map((a) => (
                          <TableCell key={a} className="px-1 [&:has([role=checkbox])]:px-1">
                            <div className="flex justify-center">
                              <Checkbox
                                aria-label={`${text.label}: ${actionLabel(a)}`}
                                disabled={!editable}
                                checked={has(r.code, a)}
                                onCheckedChange={(on) => set([[r.code, a]], on === true)}
                              />
                            </div>
                          </TableCell>
                        ))}
                        <TableCell className="border-l bg-muted/40 px-1 [&:has([role=checkbox])]:px-1">
                          <div className="flex justify-center">
                            <AllBox label={`${t("common.select_all")}: ${text.label}`} on={count(row)} total={row.length} disabled={!editable} onChange={(on) => set(row, on)} />
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}

const BLANK: Role = { id: "", name: "", enabled: true, is_default: false, permissions: {}, member_count: 0, created_at: "" };

/** One draft for name, state and permissions; a sticky bar saves or discards it. Without `role` it creates one. */
function RoleEditor({ role: saved, editable }: { role: Role | null; editable: boolean }) {
  const { t } = useI18n();
  const router = useRouter();
  const client = useQueryClient();
  const role = saved ?? BLANK;
  const [name, setName] = useState(role.name);
  const [enabled, setEnabled] = useState(role.enabled);
  const [isDefault, setIsDefault] = useState(role.is_default);
  const [draft, setDraft] = useState<Perms>(role.permissions);
  const [error, setError] = useState<string | null>(null);
  const all = "*" in role.permissions;
  const patch = {
    ...(saved && name.trim() !== role.name && { name: name.trim() }),
    ...(enabled !== role.enabled && { enabled }),
    ...(isDefault !== role.is_default && { is_default: isDefault }),
  };
  const permsDirty = !all && JSON.stringify(clean(draft)) !== JSON.stringify(clean(role.permissions));
  const dirty = (!saved && name.trim() !== "") || Object.keys(patch).length > 0 || permsDirty;

  const created = useRef(""); // set once the role exists, so a later failure knows where to go
  const save = useMutation({
    mutationFn: async () => {
      const id = saved?.id ?? (created.current = (await roles.create(name.trim())).id);
      if (Object.keys(patch).length > 0) await roles.update(id, patch);
      if (permsDirty) await roles.setPermissions(id, clean(draft));
    },
    onSuccess: () => {
      toast.success(t("common.saved"));
      void client.invalidateQueries({ queryKey: ["me"] });
      router.push("/admin/roles");
    },
    onError: (err) => {
      const message = errorMessage(err, "common.save_failed", t);
      if (!created.current) return setError(message);
      toast.error(message);
      router.replace(`/admin/roles/${created.current}`);
    },
    // A failed later call still leaves the earlier ones saved.
    onSettled: () => client.invalidateQueries({ queryKey: ["roles"] }),
  });
  useUnsavedGuard(editable && dirty && !save.isSuccess, t("roles.leave_unsaved"));
  const discard = () => {
    if (!saved) return router.push("/admin/roles");
    setName(role.name);
    setEnabled(role.enabled);
    setIsDefault(role.is_default);
    setDraft(role.permissions);
    setError(null);
  };

  return (
    <>
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <div className="space-y-6">
          <GeneralCard
            name={name}
            enabled={enabled}
            isDefault={isDefault}
            editable={editable}
            onName={setName}
            onEnabled={setEnabled}
            onDefault={(on) => { setIsDefault(on); if (on) setEnabled(true); }}
          />
          {saved && <MembersCard roleId={saved.id} editable={editable} />}
        </div>
        <PermissionsCard all={all} draft={draft} editable={editable} onChange={setDraft} />
      </div>
      {editable && (dirty || !saved) && (
        <div role="region" aria-label={t("roles.unsaved")} className="sticky bottom-4 z-10 flex flex-wrap items-center gap-3 rounded-xl border bg-card px-4 py-3 shadow-lg">
          <span className="flex-1 text-sm font-medium">{t(saved ? "roles.unsaved" : "roles.new")}</span>
          {error && <div className="basis-full"><FormError message={error} /></div>}
          <Button variant="ghost" disabled={save.isPending} onClick={discard}>{t(saved ? "common.discard" : "common.cancel")}</Button>
          <Button disabled={save.isPending || name.trim() === ""} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="size-4 animate-spin" />}
            {t(saved ? "common.save" : "common.create")}
          </Button>
        </div>
      )}
    </>
  );
}

function AddMemberDialog({ roleId, onClose }: { roleId: string; onClose: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const q = useDebounced(search.trim());
  const found = useQuery({ queryKey: ["users", "pick", q], queryFn: ({ signal }) => users.list({ page: 1, limit: 10, search: q }, signal), enabled: q.length > 1 });
  const add = useMutation({
    mutationFn: (userId: string) => roles.addMember(roleId, userId),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["roles"] });
      toast.success(t("roles.member_added"));
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader><DialogTitle>{t("roles.add_member")}</DialogTitle></DialogHeader>
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input autoFocus type="search" className="pl-9" placeholder={t("users.search")} aria-label={t("users.search")} value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
        <ul className="max-h-72 divide-y overflow-auto rounded-lg border empty:hidden">
          {found.data?.items.map((u) => (
            <li key={u.id}>
              <button type="button" disabled={add.isPending} onClick={() => add.mutate(u.id)} className="flex w-full items-center gap-3 px-3 py-2 text-left text-sm hover:bg-accent">
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{u.name}</span>
                  <span className="block truncate text-xs text-muted-foreground">{u.email}</span>
                </span>
                <Plus className="size-4 text-muted-foreground" />
              </button>
            </li>
          ))}
        </ul>
        {found.data?.items.length === 0 && <p className="text-sm text-muted-foreground">{t("users.empty")}</p>}
      </DialogContent>
    </Dialog>
  );
}

function MembersCard({ roleId, editable }: { roleId: string; editable: boolean }) {
  const { t } = useI18n();
  const { user } = useMe();
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(10);
  const [search, setSearch] = useState("");
  const q = useDebounced(search.trim());
  const [adding, setAdding] = useState(false);
  const members = useQuery({
    queryKey: ["roles", roleId, "members", page, limit, q],
    queryFn: ({ signal }) => roles.members(roleId, page, limit, q, signal),
    placeholderData: keepPreviousData,
  });
  const fallback = members.isPlaceholderData ? null : emptyPageFallback(page, limit, members.data);
  if (fallback) setPage(fallback); // the last members of the last page went away
  const remove = useMutation({
    mutationFn: (userId: string) => roles.removeMember(roleId, userId),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["roles"] });
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  // Picking a user needs the user list.
  const canAdd = editable && can(user, "users", "read");

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-2">
        <CardTitle>{t("roles.members")}</CardTitle>
        {canAdd && <Button variant="outline" size="sm" onClick={() => setAdding(true)}><Plus className="size-4" />{t("roles.add_member")}</Button>}
      </CardHeader>
      <CardContent className="space-y-3">
        {(members.data?.pagination.total_items !== 0 || q) && (
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input type="search" aria-label={t("roles.search_members")} placeholder={t("roles.search_members")} className="pl-9" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
          </div>
        )}
        {members.isPending ? (
          <ListSkeleton rows={3} />
        ) : members.isError ? (
          <QueryErrorState error={members.error} onRetry={() => void members.refetch()} />
        ) : members.data.items.length === 0 ? (
          <div className="text-sm text-muted-foreground">
            <p>{t(q ? "users.empty" : "roles.no_members")}</p>
            {!q && <p className="mt-1 text-xs">{t("roles.no_members_hint")}</p>}
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <ul className="divide-y">
              {members.data.items.map((m) => (
                <li key={m.id} className="flex items-center gap-3 px-4 py-2.5">
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium">{m.name}</div>
                    <div className="truncate text-xs text-muted-foreground">{m.email}</div>
                  </div>
                  <StatusBadge status={m.status} />
                  {editable && (
                    <Button variant="ghost" size="icon-sm" aria-label={`${t("roles.remove_member")}: ${m.name}`} disabled={remove.isPending} onClick={() => remove.mutate(m.id)}>
                      <X className="size-4" />
                    </Button>
                  )}
                </li>
              ))}
            </ul>
            <TablePagination
              page={page}
              pageSize={limit}
              totalItems={members.data.pagination.total_items}
              onPageChange={setPage}
              onPageSizeChange={(n) => { setLimit(n); setPage(1); }}
            />
          </div>
        )}
      </CardContent>
      {adding && <AddMemberDialog roleId={roleId} onClose={() => setAdding(false)} />}
    </Card>
  );
}

export default function RoleDetailPage() {
  const { t } = useI18n();
  const router = useRouter();
  const client = useQueryClient();
  const { user } = useMe();
  const { id } = useParams<{ id: string }>();
  const creating = id === "new";
  const detail = useQuery({ queryKey: ["roles", "detail", id], queryFn: () => roles.get(id), enabled: !creating });
  const role = detail.data;
  const [confirmDelete, setConfirmDelete] = useState(false);
  const editable = can(user, "roles", creating ? "create" : "update");
  const format = useFormatDate();
  useCrumb(creating ? t("roles.new") : role?.name);
  const subtitle = role && [
    role.is_default && t("roles.default"),
    `${role.member_count} ${t(role.member_count === 1 ? "roles.member_one" : "roles.members").toLowerCase()}`,
    `${t("common.created_at").toLowerCase()} ${format(role.created_at)}`,
  ].filter(Boolean).join(" · ");

  return (
    <>
      <PageHeader
        title={creating ? t("roles.new") : (role?.name ?? t("nav.roles"))}
        description={subtitle || undefined}
        action={role && !role.is_default && can(user, "roles", "delete") && (
          <Button variant="destructive" onClick={() => setConfirmDelete(true)}>{t("common.delete")}</Button>
        )}
      />
      <PageBody>
        {creating ? (
          <RoleEditor role={null} editable={editable} />
        ) : detail.isPending ? (
          <ListSkeleton rows={4} />
        ) : detail.error instanceof ApiError && detail.error.code === "NOT_FOUND" ? (
          <EmptyState title={t("error.not_found")} />
        ) : detail.isError || !role ? (
          <QueryErrorState error={detail.error} onRetry={() => void detail.refetch()} />
        ) : (
          <RoleEditor key={role.id} role={role} editable={editable} />
        )}
      </PageBody>
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t("roles.delete")}
        description={t("roles.delete_desc")}
        destructive
        confirmLabel={t("common.delete")}
        onConfirm={async () => {
          await roles.remove([id]);
          router.push("/admin/roles");
          client.removeQueries({ queryKey: ["roles", "detail", id] });
          void client.invalidateQueries({ queryKey: ["roles"] });
        }}
      />
    </>
  );
}
