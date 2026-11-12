"use client";

import { useMemo } from "react";
import { CURRENCY } from "@/lib/brand";
import { useI18n } from "@/lib/i18n/provider";
import type { Cents } from "@/lib/api/finance";

/** Fills {name} placeholders. */
export function fill(text: string, params: Record<string, string | number>): string {
  return text.replace(/\{(\w+)\}/g, (_, k: string) => String(params[k] ?? ""));
}

/** Formatters for cents and calendar dates (YYYY-MM-DD, no time zone shift). */
export function useFinanceFormat() {
  const { locale } = useI18n();
  return useMemo(() => {
    const money = new Intl.NumberFormat(locale, { style: "currency", currency: CURRENCY });
    const dayMonth = new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit", timeZone: "UTC" });
    const full = new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit", year: "numeric", timeZone: "UTC" });
    const monthYear = new Intl.DateTimeFormat(locale, { month: "long", year: "numeric", timeZone: "UTC" });
    const asDate = (d: string) => new Date(d.length === 10 ? `${d}T00:00:00Z` : d);
    return {
      money: (c: Cents) => money.format(c / 100 + 0), // + 0 turns -0 into 0
      day: (d: string) => dayMonth.format(asDate(d)),
      date: (d: string) => full.format(asDate(d)),
      month: (m: string) => {
        const s = monthYear.format(new Date(`${m}-01T00:00:00Z`));
        return s.charAt(0).toUpperCase() + s.slice(1);
      },
    };
  }, [locale]);
}

/** "1.234,56" or "1234.56" to cents; null when it is not a non-negative number. */
export function parseCents(text: string): number | null {
  const s = text.trim().replace(/[R$\s]/g, "");
  if (s === "") return 0;
  const normalized = s.includes(",") ? s.replace(/\./g, "").replace(",", ".") : s;
  const n = Number(normalized);
  return Number.isFinite(n) && n >= 0 ? Math.round(n * 100) : null;
}

/** Text color of an amount: green in, default out. */
export const amountClass = (c: Cents) => (c > 0 ? "text-emerald-700 dark:text-emerald-400" : "");
