"use client";

import Link from "next/link";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ScrollText, X } from "lucide-react";
import { PageBody, PageHeader, useFormatDate } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { TablePagination, emptyPageFallback } from "@/components/table-pagination";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { audit } from "@/lib/api/endpoints";
import type { AuditRecord } from "@/lib/api/types";
import { useGoToPage, useListParams } from "@/lib/hooks/use-list-params";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";

const KINDS = ["user.", "role.", "admin."];
const DAY = /^\d{4}-\d{2}-\d{2}$/;

/** Start of a local calendar day as an RFC 3339 time; plusDays moves it. */
function dayStart(day: string, plusDays = 0): string | undefined {
  if (!DAY.test(day)) return undefined;
  const d = new Date(`${day}T00:00:00`);
  d.setDate(d.getDate() + plusDays);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

function targetHref(r: AuditRecord): string | undefined {
  if (!r.target_id) return undefined;
  if (r.action.startsWith("user.") || r.action.startsWith("admin.")) return `/admin/users/${r.target_id}`;
  if (r.action.startsWith("role.")) return `/admin/roles/${r.target_id}`;
}

function details(d: Record<string, unknown>): string {
  return Object.entries(d).map(([k, v]) => `${k}: ${typeof v === "string" ? v : JSON.stringify(v)}`).join(" · ");
}

export default function AuditPage() {
  const { t } = useI18n();
  const format = useFormatDate();
  const [params, set] = useListParams({ page: 1, limit: 25, action: "", user: "", from: "", to: "" });
  const { page, limit, user, from, to } = params;
  const action = KINDS.includes(params.action) ? params.action : "";
  const list = useQuery({
    queryKey: ["audit", page, limit, action, user, from, to],
    queryFn: ({ signal }) => audit.list({ page, limit, action, user_id: user, from: dayStart(from), to: dayStart(to, 1) }, signal),
    placeholderData: keepPreviousData,
  });
  const fallback = list.isPlaceholderData ? null : emptyPageFallback(page, limit, list.data);
  useGoToPage(fallback, set); // the last rows of the last page went away
  const kindItems: Record<string, string> = { all: t("audit.all_actions"), ...Object.fromEntries(KINDS.map((k) => [k, t(`audit.kind.${k.slice(0, -1)}` as MessageKey)])) };
  const label = (a: string) => {
    const text = t(`audit.action.${a}` as MessageKey);
    return text.startsWith("audit.action.") ? a : text;
  };

  return (
    <>
      <PageHeader title={t("nav.audit")} description={t("audit.subtitle")} />
      <PageBody>
        <div className="flex flex-wrap items-center gap-2">
          <Select value={action || "all"} items={kindItems} onValueChange={(v) => set({ action: v === "all" ? "" : (v as string), page: 1 })}>
            <SelectTrigger className="w-44" aria-label={t("audit.action")}><SelectValue /></SelectTrigger>
            <SelectContent>
              {Object.entries(kindItems).map(([value, text]) => <SelectItem key={value} value={value}>{text}</SelectItem>)}
            </SelectContent>
          </Select>
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            {t("audit.from")}
            <Input type="date" className="w-40" value={from} max={to || undefined} onChange={(e) => set({ from: e.target.value, page: 1 })} />
          </label>
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            {t("audit.to")}
            <Input type="date" className="w-40" value={to} min={from || undefined} onChange={(e) => set({ to: e.target.value, page: 1 })} />
          </label>
          {user && (
            <Badge variant="secondary" className="gap-1">
              {t("audit.filtered_user")}
              <button type="button" aria-label={t("audit.clear_user")} className="rounded-sm hover:text-foreground" onClick={() => set({ user: "", page: 1 })}>
                <X className="size-3" />
              </button>
            </Badge>
          )}
        </div>

        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <QueryErrorState error={list.error} onRetry={() => void list.refetch()} />
        ) : list.data.items.length === 0 ? (
          <EmptyState icon={ScrollText} title={t(action || user || from || to ? "common.no_results" : "audit.empty")} />
        ) : (
          <Card className="gap-0 overflow-hidden p-0">
            <Table data-responsive>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-44">{t("audit.when")}</TableHead>
                  <TableHead>{t("audit.actor")}</TableHead>
                  <TableHead>{t("audit.action")}</TableHead>
                  <TableHead>{t("audit.target")}</TableHead>
                  <TableHead>{t("audit.details")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.data.items.map((r) => {
                  const href = targetHref(r);
                  const target = r.target_label || r.target_id?.slice(0, 8) || "-";
                  const extra = details(r.details);
                  return (
                    <TableRow key={r.id}>
                      <TableCell data-label={t("audit.when")} className="text-muted-foreground">{format(r.created_at)}</TableCell>
                      <TableCell data-label={t("audit.actor")}>
                        {r.actor_id ? (
                          <Link href={`/admin/audit?user=${r.actor_id}`} className="hover:underline" title={r.actor_email}>{r.actor_name || r.actor_email || r.actor_id.slice(0, 8)}</Link>
                        ) : (
                          <span className="text-muted-foreground">{t("audit.system")}</span>
                        )}
                      </TableCell>
                      <TableCell data-label={t("audit.action")}><Badge variant="outline" className="font-normal">{label(r.action)}</Badge></TableCell>
                      <TableCell data-label={t("audit.target")}>
                        {href ? <Link href={href} className="hover:underline">{target}</Link> : target}
                      </TableCell>
                      <TableCell data-label={t("audit.details")} className="max-w-72 truncate text-muted-foreground" title={extra}>{extra || "-"}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            <TablePagination
              page={page}
              pageSize={limit}
              totalItems={list.data.pagination.total_items}
              onPageChange={(n) => set({ page: n })}
              onPageSizeChange={(n) => set({ limit: n, page: 1 })}
            />
          </Card>
        )}
      </PageBody>
    </>
  );
}
