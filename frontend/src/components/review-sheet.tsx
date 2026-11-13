"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Loader2, SkipForward } from "lucide-react";
import { toast } from "sonner";
import { CategoryDot } from "@/components/finance";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { finance, type Category } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { amountClass, fill, useFinanceFormat } from "@/lib/finance-format";
import { merchantName } from "@/lib/merchant";
import { useI18n } from "@/lib/i18n/provider";

/** Uncategorized transactions one at a time: a click (or a digit) files it, and a rule can file the similar ones too. */
export function ReviewSheet({ range, categories, onClose }: { range: { from: string; to: string }; categories: Category[]; onClose: () => void }) {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const client = useQueryClient();
  const [skipped, setSkipped] = useState<string[]>([]);
  const [rule, setRule] = useState(true);
  const list = useQuery({
    queryKey: ["finance", "transactions", "review", range.from],
    queryFn: () => finance.transactions({ page: 1, limit: 100, ...range, category_id: "none" }),
  });
  const left = (list.data?.items ?? []).filter((tx) => !skipped.includes(tx.id));
  const tx = left[0];
  const choices = tx ? categories.filter((c) => (tx.amount < 0 ? c.kind !== "income" : c.kind !== "expense")) : [];

  const file = useMutation({
    mutationFn: async (categoryId: string) => {
      const { keyword } = await finance.setCategory(tx!.id, categoryId);
      if (rule && keyword) return (await finance.createRule(keyword, categoryId)).applied;
      return 0;
    },
    onSuccess: (applied) => {
      if (applied > 0) toast.success(fill(t("fin.review.ruled"), { n: applied }));
      void client.invalidateQueries({ queryKey: ["finance"] });
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!tx || file.isPending || e.target instanceof HTMLInputElement) return;
      const n = Number(e.key);
      if (n >= 1 && n <= 9 && choices[n - 1]) file.mutate(choices[n - 1].id);
      else if (e.key === "ArrowRight") setSkipped((s) => [...s, tx.id]);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md">
        <SheetHeader className="border-b pr-12">
          <SheetTitle>{t("fin.review.title")}</SheetTitle>
          <SheetDescription>{list.data ? fill(t("fin.review.left"), { n: left.length }) : t("fin.review.help")}</SheetDescription>
        </SheetHeader>
        <div className="grid flex-1 content-start gap-4 overflow-y-auto p-4" aria-live="polite">
          {list.isPending ? (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          ) : !tx ? (
            <div className="grid justify-items-center gap-2 py-10 text-center">
              <CheckCircle2 className="size-8 text-emerald-600" aria-hidden="true" />
              <p className="font-medium">{t("fin.review.done")}</p>
              <Button variant="outline" onClick={onClose}>{t("common.close")}</Button>
            </div>
          ) : (
            <>
              <div className="rounded-xl border p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-lg font-semibold">{merchantName(tx.description)}</p>
                    <p className="truncate text-xs text-muted-foreground">{tx.description}</p>
                    <p className="text-xs text-muted-foreground">{f.date(tx.date)} · {tx.account_name}</p>
                  </div>
                  <span className={`shrink-0 font-mono text-lg tabular-nums ${amountClass(tx.amount)}`}>{f.money(tx.amount)}</span>
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={rule} onCheckedChange={(on) => setRule(on === true)} />
                {t("fin.review.rule")}
              </label>
              <div className="grid grid-cols-2 gap-2">
                {choices.map((c, i) => (
                  <Button key={c.id} variant="outline" disabled={file.isPending} className="h-auto justify-start gap-2 py-2 text-left whitespace-normal"
                    onClick={() => file.mutate(c.id)}>
                    <CategoryDot name={c.name} className="size-6 [&_svg]:size-3.5" />
                    <span className="min-w-0 flex-1">{c.name}</span>
                    {i < 9 && <kbd className="hidden text-xs text-muted-foreground md:inline">{i + 1}</kbd>}
                  </Button>
                ))}
              </div>
              <Button variant="ghost" disabled={file.isPending} onClick={() => setSkipped((s) => [...s, tx.id])}>
                <SkipForward className="size-4" />{t("fin.review.skip")}
              </Button>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
