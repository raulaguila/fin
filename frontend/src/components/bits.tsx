"use client";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n/provider";
import type { MessageKey } from "@/lib/i18n/messages";
import type { UserStatus } from "@/lib/api/types";

export function PageHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div data-ui-page-header className="border-b px-[var(--space-page)] py-[var(--space-page-y)] sm:py-4">
      <div className="flex w-full flex-col gap-[var(--space-row)] sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
      <div className="min-w-0 max-w-full flex-1 sm:basis-64">
        <h1 className="font-heading text-xl font-semibold tracking-tight sm:text-2xl">{title}</h1>
        {description && <p className="mt-1 max-w-[72ch] text-sm text-muted-foreground">{description}</p>}
      </div>
      {action && <div className="flex max-w-full flex-wrap items-center gap-2 [&>*]:max-w-full [&>*]:flex-wrap">{action}</div>}
      </div>
    </div>
  );
}

/** Page body padding shared by every app screen. */
export function PageBody({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("w-full space-y-6 px-[var(--space-page)] py-[var(--space-page-y)]", className)}>{children}</div>;
}

const STATUS_CLS: Record<UserStatus, string> = {
  active: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300",
  pending: "bg-amber-500/15 text-amber-800 dark:text-amber-300",
  locked: "bg-red-500/15 text-red-800 dark:text-red-300",
  disabled: "bg-zinc-500/15 text-zinc-700 dark:text-zinc-300",
};

export function StatusBadge({ status }: { status: UserStatus }) {
  const { t } = useI18n();
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium", STATUS_CLS[status])}>
      <span className="size-1.5 shrink-0 rounded-full bg-current" />
      {t(`status.${status}` as MessageKey)}
    </span>
  );
}

/** Green when on, grey when off; products and roles share it. */
export function ActiveBadge({ enabled }: { enabled: boolean }) {
  const { t } = useI18n();
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium", enabled ? STATUS_CLS.active : STATUS_CLS.disabled)}>
      <span className="size-1.5 shrink-0 rounded-full bg-current" />
      {t(enabled ? "roles.enabled" : "roles.disabled")}
    </span>
  );
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? "") + (parts[1]?.[0] ?? parts[0]?.[1] ?? "")).toUpperCase();
}

/** Date and time in the user's language. */
export function useFormatDate() {
  const { locale } = useI18n();
  return (iso: string) => new Date(iso).toLocaleString(locale, { dateStyle: "medium", timeStyle: "short" });
}

/** The red box a form shows above its fields when the server refuses it. */
/** Under a field: its rule or hint while fine, in red once the value breaks a rule. */
export function FieldNote({ id, error, hint }: { id: string; error?: string | null; hint?: string }) {
  if (!error && !hint) return null;
  return <p id={id} className={cn("text-xs", error ? "text-destructive" : "text-muted-foreground")}>{error || hint}</p>;
}

export function FormError({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {message}
    </div>
  );
}

/** Icon button with a tooltip naming what it does. */
export function IconAction({ label, target, className, onClick, children }: { label: string; target: string; className?: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger render={<Button variant="ghost" size="icon-sm" className={className} aria-label={`${label}: ${target}`} onClick={onClick} />}>{children}</TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

/** The page's destructive action, kept apart at the bottom so it is never the first button in sight. */
/** className widens it to match a full-width page; the default lines up with narrow forms. */
export function DangerZone({ description, label, onClick, className }: { description: string; label: string; onClick: () => void; className?: string }) {
  const { t } = useI18n();
  return (
    <section aria-labelledby="danger-zone" className={cn("mt-8 flex max-w-xl flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/40 p-4", className)}>
      <div className="min-w-0 flex-1 space-y-1">
        <h2 id="danger-zone" className="text-sm font-medium text-destructive">{t("common.danger_zone")}</h2>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      <Button variant="destructive" onClick={onClick}>{label}</Button>
    </section>
  );
}
