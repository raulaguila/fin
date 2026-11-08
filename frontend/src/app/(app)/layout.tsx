"use client";

import { AppShell } from "@/components/app-shell";
import { MeProvider } from "@/lib/context/me";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <MeProvider>
      <AppShell>{children}</AppShell>
    </MeProvider>
  );
}
