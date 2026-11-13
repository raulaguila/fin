"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu } from "lucide-react";
import { useI18n } from "@/lib/i18n/provider";
import { MAIN_NAV, inSection } from "@/lib/navigation";
import { cn } from "@/lib/utils";

/** Phone navigation within thumb reach: the main items, and "more" for settings and administration. */
export function BottomNav({ onMore }: { onMore: () => void }) {
  const { t } = useI18n();
  const pathname = usePathname();
  const item = "flex min-w-0 flex-1 flex-col items-center gap-0.5 py-2 text-[11px]";
  return (
    <nav aria-label={t("a11y.nav_menu")} className="fixed inset-x-0 bottom-0 z-30 flex border-t bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden">
      {MAIN_NAV.map((nav) => {
        const current = inSection(pathname, nav);
        const Icon = nav.icon;
        return (
          <Link key={nav.href} href={nav.href} aria-current={current ? "page" : undefined}
            className={cn(item, current ? "font-medium text-primary" : "text-muted-foreground")}>
            <Icon className="size-5" aria-hidden="true" />
            <span className="max-w-full truncate">{t(nav.key)}</span>
          </Link>
        );
      })}
      <button type="button" onClick={onMore} className={cn(item, "text-muted-foreground")}>
        <Menu className="size-5" aria-hidden="true" />
        <span>{t("nav.more")}</span>
      </button>
    </nav>
  );
}
