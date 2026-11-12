import { ArrowLeftRight, Bot, HandCoins, KeyRound, LayoutDashboard, Plug, ScrollText, Tags, Target, Users, type LucideIcon } from "lucide-react";
import type { Action } from "@/lib/api/types";
import type { MessageKey } from "@/lib/i18n/messages";

export interface NavItem {
  href: string;
  key: MessageKey;
  icon: LucideIcon;
  /** Shown only when the user may do this; absent means every signed in user. */
  needs?: [resource: string, action: Action];
}

export const MAIN_NAV: NavItem[] = [
  { href: "/", key: "nav.dashboard", icon: LayoutDashboard },
  { href: "/transactions", key: "nav.transactions", icon: ArrowLeftRight },
  { href: "/budget", key: "nav.budget", icon: Target },
  { href: "/people", key: "nav.people", icon: HandCoins },
  { href: "/categories", key: "nav.categories", icon: Tags },
  { href: "/connections", key: "nav.connections", icon: Plug },
  { href: "/assistant", key: "nav.assistant", icon: Bot },
];

export const ADMIN_NAV: NavItem[] = [
  { href: "/admin/users", key: "nav.users", icon: Users, needs: ["users", "read"] },
  { href: "/admin/roles", key: "nav.roles", icon: KeyRound, needs: ["roles", "read"] },
  { href: "/admin/audit", key: "nav.audit", icon: ScrollText, needs: ["audit", "read"] },
];

export function isActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(href + "/");
}
