"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useI18n } from "@/lib/i18n/provider";
import { isActive, type NavItem } from "@/lib/navigation";
import { cn } from "@/lib/utils";

/** Tabs between the pages of one menu item, so related screens read as one place. */
export function SectionTabs({ item }: { item: NavItem }) {
  const { t } = useI18n();
  const pathname = usePathname();
  if (!item.tabs) return null;
  return (
    <nav aria-label={t(item.key)} className="flex gap-1 overflow-x-auto overflow-y-hidden px-[var(--space-page)] pt-2 shadow-[inset_0_-1px_0_var(--color-border)] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {item.tabs.map((tab) => {
        const current = isActive(pathname, tab.href);
        return (
          <Link
            key={tab.href}
            href={tab.href}
            aria-current={current ? "page" : undefined}
            className={cn(
              "shrink-0 border-b-2 px-3 py-2 text-sm whitespace-nowrap transition-colors",
              current ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {t(tab.key)}
          </Link>
        );
      })}
    </nav>
  );
}
