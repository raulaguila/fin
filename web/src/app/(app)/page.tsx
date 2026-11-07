"use client";

import Link from "next/link";
import { PageBody, PageHeader } from "@/components/bits";
import { Card } from "@/components/ui/card";
import { useMe } from "@/lib/context/me";
import { useI18n } from "@/lib/i18n/provider";
import { ADMIN_NAV, MAIN_NAV } from "@/lib/navigation";
import { can } from "@/lib/permissions";

export default function HomePage() {
  const { t } = useI18n();
  const { user } = useMe();
  const items = [...MAIN_NAV, ...ADMIN_NAV].filter((item) => item.href !== "/" && (!item.needs || can(user, ...item.needs)));
  return (
    <>
      <PageHeader title={user ? `${t("home.greeting")}, ${user.name.split(" ")[0]}` : t("nav.home")} description={t("home.subtitle")} />
      <PageBody>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((item) => (
            <Link key={item.href} href={item.href} className="group">
              <Card className="h-full gap-3 p-5 transition-colors group-hover:border-primary/50">
                <span className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <item.icon className="size-5" />
                </span>
                <span className="font-medium">{t(item.key)}</span>
              </Card>
            </Link>
          ))}
        </div>
      </PageBody>
    </>
  );
}
