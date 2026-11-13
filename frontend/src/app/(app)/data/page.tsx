"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Download, Loader2, Upload, XCircle } from "lucide-react";
import { toast } from "sonner";
import { PageBody, PageHeader, useFormatDate } from "@/components/bits";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, type AccountHealth, type ArchiveResult, type DataHealth } from "@/lib/api/finance";
import { archiveParts } from "@/lib/archive";
import { errorMessage } from "@/lib/errors";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const STALE_DAYS = 7;
/** Below R$ 1 a gap is rounding at the bank, not missing data. */
const GAP_MIN = 100;

const daysSince = (date: string) => Math.floor((Date.now() - new Date(`${date}T00:00:00`).getTime()) / 86_400_000);

function Status({ ok, warn, children }: { ok: boolean; warn?: boolean; children: React.ReactNode }) {
  const Icon = ok ? CheckCircle2 : warn ? AlertTriangle : XCircle;
  return (
    <div className="flex items-start gap-2 text-sm">
      <Icon className={cn("mt-0.5 size-4 shrink-0", ok ? "text-emerald-600" : warn ? "text-amber-600" : "text-red-600")} aria-hidden="true" />
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function AccountRow({ a }: { a: AccountHealth }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const accept = useMutation({
    mutationFn: () => finance.resetBalanceCheck(a.id),
    onSuccess: () => void client.invalidateQueries({ queryKey: ["finance", "health"] }),
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const age = a.last_date ? daysSince(a.last_date) : null;
  // A bank account always moves; a card or an unused account can rest for weeks.
  const stale = !a.manual && a.type === "BANK" && age !== null && age > STALE_DAYS;
  const gap = a.gap !== null && Math.abs(a.gap) >= GAP_MIN ? a.gap : 0;
  return (
    <li className="grid gap-1.5 py-3">
      <Status ok={!stale && !gap} warn>
        <p className="font-medium">{a.institution} · {a.name}</p>
        <p className="text-xs text-muted-foreground">
          {a.last_date ? fill(t("fin.data.last_tx"), { d: f.date(a.last_date), n: age ?? 0 }) : t("fin.data.no_tx")}
          {stale && ` · ${t("fin.data.stale")}`}
        </p>
        {gap !== 0 && (
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <p className="text-xs text-amber-800 dark:text-amber-300">
              {fill(t(gap < 0 ? "fin.data.gap_out" : "fin.data.gap_in"), { v: f.money(Math.abs(gap)), d: f.date(a.check_since!.slice(0, 10)) })}
            </p>
            <Button size="sm" variant="outline" disabled={accept.isPending} onClick={() => accept.mutate()}>{t("fin.data.gap_accept")}</Button>
          </div>
        )}
      </Status>
    </li>
  );
}

const BACKUP_STEPS: { kind: DataHealth["backups"][number]["kind"]; label: MessageKey; missing: MessageKey }[] = [
  { kind: "dump", label: "fin.data.backup_dump", missing: "fin.data.backup_dump_none" },
  { kind: "copy", label: "fin.data.backup_copy", missing: "fin.data.backup_copy_none" },
  { kind: "restore_check", label: "fin.data.backup_check", missing: "fin.data.backup_check_none" },
];

function Health({ h }: { h: DataHealth }) {
  const { t } = useI18n();
  const when = useFormatDate();
  const client = useQueryClient();
  const pair = useMutation({
    mutationFn: finance.pairTransfers,
    onSuccess: (r) => {
      toast.success(fill(t("fin.data.paired_now"), { n: r.paired }));
      void client.invalidateQueries({ queryKey: ["finance"] });
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const runs = new Map(h.backups.map((b) => [b.kind, b]));
  const [now] = useState(Date.now);
  return (
    <>
      <Card className="gap-1 p-4">
        <h2 className="text-sm font-medium">{t("fin.data.accounts")}</h2>
        <p className="text-xs text-muted-foreground">{t("fin.data.accounts_help")}</p>
        <ul className="divide-y">{h.accounts.map((a) => <AccountRow key={a.id} a={a} />)}</ul>
      </Card>

      <Card className="gap-3 p-4">
        <h2 className="text-sm font-medium">{t("fin.data.classification")}</h2>
        <Status ok={h.uncategorized === 0} warn>
          {h.uncategorized === 0 ? t("fin.data.uncategorized_none") : (
            <Link href="/transactions?category=none" className="hover:underline">{fill(t("fin.data.uncategorized"), { n: h.uncategorized })}</Link>
          )}
        </Status>
        <Status ok>
          <div className="flex flex-wrap items-center gap-2">
            <span>{fill(t("fin.data.paired"), { n: h.paired })}</span>
            <Button size="sm" variant="outline" disabled={pair.isPending} onClick={() => pair.mutate()}>
              {pair.isPending && <Loader2 className="size-4 animate-spin" />}{t("fin.data.pair_now")}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">{t("fin.data.paired_help")}</p>
        </Status>
      </Card>

      <Card className="gap-3 p-4">
        <h2 className="text-sm font-medium">{t("fin.data.backups")}</h2>
        {BACKUP_STEPS.map((s) => {
          const run = runs.get(s.kind);
          // A dump or a check older than two days means the backup service stopped.
          const old = run && now - new Date(run.at).getTime() > (s.kind === "restore_check" ? 8 : 2) * 86_400_000;
          return (
            <Status key={s.kind} ok={!!run?.ok && !old} warn={!run || old}>
              <p>{t(s.label)}</p>
              <p className="text-xs text-muted-foreground">{run ? `${when(run.at)} · ${run.detail}` : t(s.missing)}</p>
            </Status>
          );
        })}
      </Card>
    </>
  );
}

const RESULT_KEYS: (keyof ArchiveResult)[] = ["categories", "rules", "goals", "subscriptions", "debts", "accounts", "transactions", "overrides"];

function Archive() {
  const { t } = useI18n();
  const client = useQueryClient();
  const input = useRef<HTMLInputElement>(null);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const exportFile = useMutation({
    mutationFn: finance.exportArchive,
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const importFile = async (file: File) => {
    let parts: Record<string, unknown>[];
    try {
      parts = archiveParts(JSON.parse(await file.text()));
    } catch {
      toast.error(t("fin.data.import_bad"));
      return;
    }
    const sum = Object.fromEntries(RESULT_KEYS.map((k) => [k, 0])) as unknown as ArchiveResult;
    setProgress({ done: 0, total: parts.length });
    try {
      for (const [i, part] of parts.entries()) {
        const r = await finance.importArchive(part);
        for (const k of RESULT_KEYS) sum[k] += r[k];
        setProgress({ done: i + 1, total: parts.length });
      }
      toast.success(fill(t("fin.data.import_done"), { ...sum }));
    } catch (err) {
      toast.error(errorMessage(err, "fin.data.import_failed", t));
    } finally {
      setProgress(null);
      void client.invalidateQueries({ queryKey: ["finance"] });
    }
  };
  return (
    <Card className="gap-3 p-4">
      <h2 className="text-sm font-medium">{t("fin.data.archive")}</h2>
      <p className="text-sm text-muted-foreground">{t("fin.data.archive_help")}</p>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" disabled={exportFile.isPending} onClick={() => exportFile.mutate()}>
          {exportFile.isPending ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}{t("fin.data.export")}
        </Button>
        <Button variant="outline" disabled={!!progress} onClick={() => input.current?.click()}>
          {progress ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
          {progress ? fill(t("fin.data.importing"), { n: progress.done, total: progress.total }) : t("fin.data.import")}
        </Button>
        <input ref={input} type="file" accept="application/json,.json" className="hidden" aria-label={t("fin.data.import")}
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = "";
            if (file) void importFile(file);
          }} />
      </div>
      <p className="text-xs text-muted-foreground">{t("fin.data.archive_note")}</p>
    </Card>
  );
}

export default function DataPage() {
  const { t } = useI18n();
  const health = useQuery({ queryKey: ["finance", "health"], queryFn: finance.health });
  return (
    <>
      <PageHeader title={t("nav.data")} description={t("fin.data.subtitle")} />
      <PageBody>
        {health.isPending ? (
          <ListSkeleton />
        ) : health.isError ? (
          <QueryErrorState error={health.error} onRetry={() => void health.refetch()} />
        ) : (
          <Health h={health.data} />
        )}
        <Archive />
      </PageBody>
    </>
  );
}
