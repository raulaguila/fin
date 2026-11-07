"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { ActiveBadge, PageBody, PageHeader, StatusBadge, useFormatDate } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { roles, users } from "@/lib/api/endpoints";
import type { UserDetails } from "@/lib/api/types";
import { errorMessage } from "@/lib/errors";
import { useMe } from "@/lib/context/me";
import { useUnsavedGuard } from "@/lib/hooks/use-unsaved-guard";
import { useCrumb } from "@/lib/store/crumb";
import { useI18n } from "@/lib/i18n/provider";
import { can } from "@/lib/permissions";

/** Check boxes for every role the API lists; Save replaces the user's roles in one call. */
function RolesCard({ user }: { user: UserDetails }) {
  const { t } = useI18n();
  const { user: me } = useMe();
  const client = useQueryClient();
  const router = useRouter();
  const manage = can(me, "roles", "read") && can(me, "roles", "update");
  // shortcut: shows the first 100 roles (the API page cap); add search here if a deployment grows past that.
  const all = useQuery({ queryKey: ["roles", "list", "all"], queryFn: () => roles.list({ page: 1, limit: 100 }), enabled: manage });
  const saved = user.roles.map((r) => r.id).sort();
  const [draft, setDraft] = useState(saved);
  const dirty = JSON.stringify([...draft].sort()) !== JSON.stringify(saved);
  useUnsavedGuard(dirty, t("roles.leave_unsaved"));
  const save = useMutation({
    mutationFn: () => users.setRoles(user.id, draft),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["users", "detail", user.id] });
      void client.invalidateQueries({ queryKey: ["roles"] });
      if (user.id === me?.id) void client.invalidateQueries({ queryKey: ["me"] });
      toast.success(t("common.saved"));
      router.push("/admin/users");
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const toggle = (id: string, on: boolean) => setDraft((d) => (on ? [...d, id] : d.filter((x) => x !== id)));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("nav.roles")}</CardTitle>
        {manage && <CardDescription>{t("users.roles_hint")}</CardDescription>}
      </CardHeader>
      <CardContent className="space-y-4">
        {!manage ? (
          <div className="flex flex-wrap gap-1.5">
            {user.roles.length === 0 ? <span className="text-sm text-muted-foreground">-</span> : user.roles.map((r) => (
              <Badge key={r.id} variant="secondary" render={<Link href={`/admin/roles/${r.id}`} />}>{r.name}</Badge>
            ))}
          </div>
        ) : all.isPending ? (
          <ListSkeleton rows={3} />
        ) : all.isError ? (
          <QueryErrorState onRetry={() => void all.refetch()} />
        ) : (
          <>
            <ul className="divide-y overflow-hidden rounded-lg border">
              {all.data.items.map((r) => (
                <li key={r.id}>
                  <label className="flex cursor-pointer items-center gap-3 px-4 py-2.5 text-sm hover:bg-accent">
                    <Checkbox checked={draft.includes(r.id)} disabled={save.isPending} onCheckedChange={(on) => toggle(r.id, on === true)} />
                    <span className="min-w-0 flex-1 truncate font-medium">{r.name}</span>
                    {r.is_default && <Badge variant="outline">{t("roles.default")}</Badge>}
                    {!r.enabled && <ActiveBadge enabled={false} />}
                  </label>
                </li>
              ))}
            </ul>
            <div className="flex justify-end gap-2">
              {dirty && <Button variant="ghost" disabled={save.isPending} onClick={() => setDraft(saved)}>{t("common.discard")}</Button>}
              <Button disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
                {save.isPending && <Loader2 className="size-4 animate-spin" />}
                {t("common.save")}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default function UserDetailPage() {
  const { t } = useI18n();
  const format = useFormatDate();
  const { user: me } = useMe();
  const client = useQueryClient();
  const { id } = useParams<{ id: string }>();
  const detail = useQuery({ queryKey: ["users", "detail", id], queryFn: () => users.get(id) });
  const [confirm, setConfirm] = useState(false);

  const u = detail.data;
  useCrumb(u?.name);
  const disabled = u?.status === "disabled";
  // The API refuses changing yourself; hide the button instead of showing an error.
  const canToggle = u && can(me, "users", "update") && u.id !== me?.id;

  return (
    <>
      <PageHeader
        title={u?.name ?? t("nav.users")}
        description={u?.email}
        action={canToggle && (
          <Button variant={disabled ? "default" : "destructive"} onClick={() => setConfirm(true)}>
            {t(disabled ? "users.enable" : "users.disable")}
          </Button>
        )}
      />
      <PageBody>
        {detail.isPending ? (
          <ListSkeleton rows={3} />
        ) : detail.isError ? (
          <QueryErrorState onRetry={() => void detail.refetch()} />
        ) : (
          <div className="grid items-start gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>{t("users.details")}</CardTitle></CardHeader>
            <CardContent>
              <dl className="grid gap-x-6 gap-y-4 text-sm sm:grid-cols-[8rem_1fr]">
                <dt className="text-muted-foreground">{t("users.status")}</dt>
                <dd><StatusBadge status={detail.data.status} /></dd>
                <dt className="text-muted-foreground">{t("common.created_at")}</dt>
                <dd>{format(detail.data.created_at)}</dd>
                <dt className="text-muted-foreground">{t("common.updated_at")}</dt>
                <dd>{format(detail.data.updated_at)}</dd>
              </dl>
            </CardContent>
          </Card>
          <RolesCard user={detail.data} />
          </div>
        )}
      </PageBody>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={t(disabled ? "users.enable" : "users.disable")}
        description={t(disabled ? "users.enable_desc" : "users.disable_desc")}
        destructive={!disabled}
        onConfirm={async () => {
          await (disabled ? users.enable(id) : users.disable(id));
          await client.invalidateQueries({ queryKey: ["users"] });
          toast.success(t("common.saved"));
        }}
      />
    </>
  );
}
