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
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-[var(--space-row)] sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
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
  return <div className={cn("mx-auto w-full max-w-6xl space-y-6 px-[var(--space-page)] py-[var(--space-page-y)]", className)}>{children}</div>;
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
