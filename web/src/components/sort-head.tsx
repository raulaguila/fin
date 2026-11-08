"use client";

import { ArrowDown, ArrowUp } from "lucide-react";
import { TableHead } from "@/components/ui/table";
import type { SortOrder } from "@/lib/api/types";

/** A column header that sorts the list; a new column starts ascending, dates newest first. */
export function SortHead({ field, label, sort, order, onSort, className }: {
  field: string;
  label: string;
  sort: string;
  order: SortOrder;
  onSort: (sort: string, order: SortOrder) => void;
  className?: string;
}) {
  const active = sort === field;
  const next = active ? (order === "asc" ? "desc" : "asc") : field.endsWith("_at") ? "desc" : "asc";
  return (
    <TableHead className={className} aria-sort={active ? (order === "asc" ? "ascending" : "descending") : undefined}>
      <button type="button" className="inline-flex items-center gap-1 hover:text-foreground" onClick={() => onSort(field, next)}>
        {label}
        {active && (order === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />)}
      </button>
    </TableHead>
  );
}
