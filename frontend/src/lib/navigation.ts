import { ArrowLeftRight, FileText, KeyRound, LayoutDashboard, ScrollText, Settings, Target, Users, type LucideIcon } from "lucide-react";
import type { Action } from "@/lib/api/types";
import type { MessageKey } from "@/lib/i18n/messages";

export interface NavTab {
  href: string;
  key: MessageKey;
}

export interface NavItem {
  href: string;
  key: MessageKey;
  icon: LucideIcon;
  /** Shown only when the user may do this; absent means every signed in user. */
  needs?: [resource: string, action: Action];
  /** Pages grouped under this item, shown as tabs on each of them. */
  tabs?: NavTab[];
}

export const MAIN_NAV: NavItem[] = [
  { href: "/", key: "nav.dashboard", icon: LayoutDashboard },
  { href: "/transactions", key: "nav.transactions", icon: ArrowLeftRight },
  {
    href: "/budget", key: "nav.planning", icon: Target,
    tabs: [
      { href: "/budget", key: "nav.budget" },
      { href: "/subscriptions", key: "nav.subscriptions" },
      { href: "/goals", key: "nav.goals" },
      { href: "/calendar", key: "nav.calendar" },
    ],
  },
  {
    href: "/report", key: "nav.reports", icon: FileText,
    tabs: [
      { href: "/report", key: "nav.report" },
      { href: "/people", key: "nav.people" },
    ],
  },
];

export const SETTINGS_NAV: NavItem = {
  href: "/connections", key: "nav.settings", icon: Settings,
  tabs: [
    { href: "/connections", key: "nav.connections" },
    { href: "/categories", key: "nav.categories" },
    { href: "/profile", key: "nav.profile" },
  ],
};

export const ADMIN_NAV: NavItem[] = [
  { href: "/admin/users", key: "nav.users", icon: Users, needs: ["users", "read"] },
  { href: "/admin/roles", key: "nav.roles", icon: KeyRound, needs: ["roles", "read"] },
  { href: "/admin/audit", key: "nav.audit", icon: ScrollText, needs: ["audit", "read"] },
];

export function isActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(href + "/");
}

/** True on the item's own page or on any of its tabs. */
export const inSection = (pathname: string, item: NavItem) =>
  (item.tabs ?? [item]).some((tab) => isActive(pathname, tab.href));
