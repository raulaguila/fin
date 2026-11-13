"use client";

import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { finance } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";

/** A success toast for a category change, with a button that puts it back (and drops the rule that made it). */
export function useUndoToast() {
  const { t } = useI18n();
  const client = useQueryClient();
  return (message: string, undoId: string | undefined, changed: number) => {
    if (!undoId || changed === 0) {
      toast.success(message);
      return;
    }
    toast.success(message, {
      duration: 8_000,
      action: {
        label: t("fin.undo.action"),
        onClick: () => {
          finance.undo(undoId)
            .then((r) => toast.success(fill(t("fin.undo.done"), { n: r.restored })))
            .catch((err) => toast.error(errorMessage(err, "common.save_failed", t)))
            .finally(() => void client.invalidateQueries({ queryKey: ["finance"] }));
        },
      },
    });
  };
}
