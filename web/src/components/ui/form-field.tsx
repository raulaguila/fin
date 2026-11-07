import type { ComponentProps, ReactNode } from "react";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

/** Share label, control and help rows so wrapped labels never offset controls. */
export function FormFieldGrid({ className, columns = 2, ...props }: ComponentProps<"div"> & { columns?: 2 | 3 }) {
  return <div data-slot="form-field-grid" className={cn(
    "grid min-w-0 grid-cols-1 gap-x-4 gap-y-4 sm:gap-y-0 sm:*:data-[slot=form-field]:row-span-3 sm:*:data-[slot=form-field]:grid-rows-subgrid",
    columns === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2", className,
  )} {...props} />;
}

export function FormField({ id, label, description, children, className, labelClassName }: {
  id: string;
  label: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  className?: string;
  labelClassName?: string;
}) {
  return <div data-slot="form-field" className={cn("grid min-w-0 grid-rows-[auto_auto_1fr] content-start gap-0", className)}>
    <Label htmlFor={id} className={cn("self-end pb-2", labelClassName)}>{label}</Label>
    <div data-slot="form-control" className="min-w-0 self-start *:data-[slot=select-trigger]:w-full *:data-[slot=dropdown-select-trigger]:w-full">{children}</div>
    <div data-slot="form-description" className={cn("min-w-0 space-y-2 text-xs text-muted-foreground", description && "pt-2")} aria-hidden={description ? undefined : true}>{description}</div>
  </div>;
}
