"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Landmark, Plug, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useAccountLabel, useFinanceStatus } from "@/components/finance";
import { PageBody, PageHeader, useFormatDate } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, type Connection, type ConnectionStatus } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import type { MessageKey } from "@/lib/i18n/messages";
import { cn } from "@/lib/utils";

// Connector ids the widget lists; empty lists every one. A Pluggy trial account
// reaches real banks only through MeuPluggy (200), next to the sandbox (2).
const CONNECTORS = (process.env.NEXT_PUBLIC_PLUGGY_CONNECTORS ?? "").split(",").map(Number).filter((n) => n > 0);

const STATUS_CLS: Record<ConnectionStatus, string> = {
  ok: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300",
  syncing: "bg-sky-500/15 text-sky-800 dark:text-sky-300",
  error: "bg-red-500/15 text-red-800 dark:text-red-300",
  login_required: "bg-amber-500/15 text-amber-800 dark:text-amber-300",
};

/**
 * Opens the Pluggy Connect widget. With an item id it renews that connection.
 * The SDK touches window, so it loads only on click.
 */
function usePluggyConnect() {
  const { t } = useI18n();
  const client = useQueryClient();
  const [opening, setOpening] = useState(false);
  const open = async (connection?: Connection) => {
    setOpening(true);
    try {
      const [{ PluggyConnect }, { access_token }] = await Promise.all([import("pluggy-connect-sdk"), finance.connectToken(connection?.id)]);
      const widget = new PluggyConnect({
        connectToken: access_token,
        includeSandbox: process.env.NODE_ENV !== "production",
        updateItem: connection?.item_id,
        connectorIds: CONNECTORS.length ? CONNECTORS : undefined,
        onSuccess: async ({ item }) => {
          try {
            if (connection) await finance.sync(connection.id);
            else {
              const added = await finance.addConnection(item.id);
              toast.success(fill(t("fin.conn.added"), { b: added.institution }));
            }
          } catch (err) {
            toast.error(errorMessage(err, "common.save_failed", t));
          }
          await client.invalidateQueries({ queryKey: ["finance"] });
        },
        onClose: () => void widget.destroy().catch(() => {}),
      });
      await widget.init();
    } catch (err) {
      toast.error(errorMessage(err, "fin.conn.widget_failed", t));
    } finally {
      setOpening(false);
    }
  };
  return { open, opening };
}

function ConnectionCard({ c, onReconnect, onRemove }: { c: Connection; onReconnect: () => void; onRemove: () => void }) {
  const label = useAccountLabel();
  const { t } = useI18n();
  const f = useFinanceFormat();
  const formatDate = useFormatDate();
  const client = useQueryClient();
  const sync = useMutation({
    mutationFn: () => finance.sync(c.id),
    onSuccess: () => {
      toast.success(t("fin.conn.sync_started"));
      void client.invalidateQueries({ queryKey: ["finance", "connections"] });
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const needsLogin = c.status === "login_required" || c.status === "error";
  return (
    <Card className="gap-3 p-4">
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted">
          {/* eslint-disable-next-line @next/next/no-img-element -- a remote logo, not worth the image optimizer */}
          {c.image_url ? <img src={c.image_url} alt="" className="size-8 object-contain" /> : <Landmark className="size-5 text-muted-foreground" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="truncate font-medium">{c.institution || "—"}</h2>
            <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium", STATUS_CLS[c.status])}>
              <span className="size-1.5 rounded-full bg-current" />
              {t(`fin.conn.status.${c.status}` as MessageKey)}
            </span>
          </div>
          <p className="text-xs text-muted-foreground">
            {c.last_sync_at ? fill(t("fin.conn.last_sync"), { d: formatDate(c.last_sync_at) }) : t("fin.conn.never")}
            {c.consent_expires_at && <> · {fill(t("fin.conn.consent"), { d: f.date(c.consent_expires_at) })}</>}
          </p>
          {c.error && <p className="mt-1 text-xs text-red-700 dark:text-red-400">{c.error}</p>}
        </div>
      </div>
      {c.accounts.length > 0 && (
        <ul className="divide-y rounded-lg border text-sm">
          {c.accounts.map((a) => (
            <li key={a.id} className="flex items-center justify-between gap-3 px-3 py-2">
              <span className="min-w-0 truncate">{label(a)}</span>
              <span className="shrink-0 text-right font-mono tabular-nums">
                {a.type === "CREDIT" ? fill(t("fin.conn.bill"), { v: f.money(a.balance) }) : f.money(a.balance)}
                {a.type === "CREDIT" && a.available !== null && <span className="block text-xs text-muted-foreground">{fill(t("fin.conn.available"), { v: f.money(a.available) })}</span>}
              </span>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        {needsLogin ? (
          <Button size="sm" onClick={onReconnect}><Plug className="size-4" />{t("fin.conn.reconnect")}</Button>
        ) : (
          <Button size="sm" variant="outline" disabled={sync.isPending || c.status === "syncing"} onClick={() => sync.mutate()}>
            <RefreshCw className={cn("size-4", c.status === "syncing" && "animate-spin")} />{t("fin.conn.sync_now")}
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={onRemove}><Trash2 className="size-4" />{t("fin.conn.remove")}</Button>
      </div>
    </Card>
  );
}

export default function ConnectionsPage() {
  const { t } = useI18n();
  const client = useQueryClient();
  const status = useFinanceStatus();
  const { open, opening } = usePluggyConnect();
  const [removing, setRemoving] = useState<Connection | null>(null);
  const connections = useQuery({
    queryKey: ["finance", "connections"],
    queryFn: finance.connections,
    // While a bank is syncing, look again every few seconds.
    refetchInterval: (q) => (q.state.data?.items.some((c) => c.status === "syncing") ? 5_000 : false),
  });
  const enabled = status.data?.open_finance !== false;
  const items = connections.data?.items ?? [];

  return (
    <>
      <PageHeader title={t("nav.connections")} description={t("fin.conn.subtitle")}
        action={enabled && <Button disabled={opening} onClick={() => void open()}><Plug className="size-4" />{t("fin.connect_bank")}</Button>} />
      <PageBody>
        {!enabled ? (
          <EmptyState icon={Plug} title={t("fin.of_disabled_title")} description={t("fin.of_disabled_body")} />
        ) : connections.isPending ? (
          <ListSkeleton />
        ) : connections.isError ? (
          <QueryErrorState error={connections.error} onRetry={() => void connections.refetch()} />
        ) : items.length === 0 ? (
          <EmptyState icon={Landmark} title={t("fin.no_connections_title")} description={`${t("fin.no_connections_body")} ${t("fin.conn.read_only")}`}
            action={<Button disabled={opening} onClick={() => void open()}>{t("fin.connect_bank")}</Button>} />
        ) : (
          <div className="grid gap-4 lg:grid-cols-2">
            {items.map((c) => <ConnectionCard key={c.id} c={c} onReconnect={() => void open(c)} onRemove={() => setRemoving(c)} />)}
          </div>
        )}
        <ConfirmDialog
          open={!!removing}
          onOpenChange={(o) => !o && setRemoving(null)}
          title={fill(t("fin.conn.remove_title"), { b: removing?.institution ?? "" })}
          description={t("fin.conn.remove_body")}
          destructive
          confirmLabel={t("fin.conn.remove")}
          onConfirm={async () => {
            if (!removing) return;
            await finance.removeConnection(removing.id);
            setRemoving(null);
            await client.invalidateQueries({ queryKey: ["finance"] });
          }}
        />
      </PageBody>
    </>
  );
}
