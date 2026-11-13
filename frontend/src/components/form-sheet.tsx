"use client";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useI18n } from "@/lib/i18n/provider";

/**
 * A form in a side drawer: full screen on a phone, a wide column on a desktop.
 * Header and buttons stay put while the fields scroll, so long forms and long
 * names fit where a centered dialog cramped them.
 */
export function FormSheet({ open = true, onClose, title, description, submitLabel, canSubmit, pending, onSubmit, children }: {
  open?: boolean;
  onClose: () => void;
  title: string;
  description?: React.ReactNode;
  submitLabel: string;
  canSubmit: boolean;
  pending?: boolean;
  onSubmit: () => void;
  children: React.ReactNode;
}) {
  const { t } = useI18n();
  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md">
        <SheetHeader className="border-b pr-12">
          <SheetTitle>{title}</SheetTitle>
          {description && <SheetDescription>{description}</SheetDescription>}
        </SheetHeader>
        <form className="flex min-h-0 flex-1 flex-col" onSubmit={(e) => { e.preventDefault(); if (canSubmit && !pending) onSubmit(); }}>
          <div className="grid flex-1 content-start gap-4 overflow-y-auto p-4">{children}</div>
          <SheetFooter className="flex-row justify-end border-t">
            <Button type="button" variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
            <Button type="submit" disabled={!canSubmit || pending}>{submitLabel}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
