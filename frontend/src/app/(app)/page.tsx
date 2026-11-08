"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { UserRoundPlus, type LucideIcon } from "lucide-react";
import { PageBody, PageHeader } from "@/components/bits";
import { Card } from "@/components/ui/card";
import { products, roles, users } from "@/lib/api/endpoints";
import { useMe } from "@/lib/context/me";
import { useI18n } from "@/lib/i18n/provider";
import { ADMIN_NAV, MAIN_NAV } from "@/lib/navigation";
import { can } from "@/lib/permissions";

type Total = { key: readonly unknown[]; fn: () => Promise<{ pagination: { total_items: number } }> };

// One row of each list is enough: the total comes back in the pagination.
const totals: Record<string, Total> = {
  "/products": { key: ["products", "total"], fn: () => products.list({ page: 1, limit: 1 }) },
  "/admin/users": { key: ["users", "total"], fn: () => users.list({ page: 1, limit: 1 }) },
  "/admin/roles": { key: ["roles", "list", "total"], fn: () => roles.list({ page: 1, limit: 1 }) },
};

function StatCard({ href, label, icon: Icon, total }: { href: string; label: string; icon: LucideIcon; total?: Total }) {
  const count = useQuery({ queryKey: total?.key ?? ["none"], queryFn: () => total!.fn(), enabled: !!total });
  return (
    <Link href={href} className="group rounded-lg focus-visible:outline-2 focus-visible:outline-ring">
      <Card className="h-full gap-2 p-4 transition-colors group-hover:bg-muted">
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm text-muted-foreground">{label}</span>
          <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
        </div>
        <span className="text-2xl font-semibold tabular-nums">{total ? (count.data?.pagination.total_items ?? "—") : "→"}</span>
      </Card>
    </Link>
  );
}

export default function HomePage() {
  const { t } = useI18n();
  const { user } = useMe();
  const items = [...MAIN_NAV, ...ADMIN_NAV].filter((item) => item.href !== "/" && (!item.needs || can(user, ...item.needs)));
  const pending: Total = {
    key: ["users", "total", "pending"],
    fn: () => users.list({ page: 1, limit: 1, status: "pending" }),
  };
  return (
    <>
      <PageHeader title={user ? `${t("home.greeting")}, ${user.name.split(" ")[0]}` : t("nav.home")} description={t("home.subtitle")} />
      <PageBody>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((item) => <StatCard key={item.href} href={item.href} label={t(item.key)} icon={item.icon} total={totals[item.href]} />)}
          {can(user, "users", "read") && <StatCard href="/admin/users?status=pending" label={t("home.pending")} icon={UserRoundPlus} total={pending} />}
        </div>
      </PageBody>
    </>
  );
}
