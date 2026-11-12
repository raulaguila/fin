"use client";

import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileUp } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { finance } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill } from "@/lib/finance-format";
import { useI18n } from "@/lib/i18n/provider";
import { parseStatement } from "@/lib/statement";

const BATCH = 500; // about 50 KB of JSON: under the 100 KB request limit of the proxy and the API
const MAX_FILE = 5 * 1024 * 1024;

/** Picks an OFX or CSV file, parses it here and sends the lines in batches. */
export function ImportFileButton({ accountId }: { accountId: string }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const pick = async (file: File) => {
    if (file.size > MAX_FILE) return toast.error(t("fin.imp.too_big"));
    setBusy(true);
    try {
      let text = await file.text();
      // Many Brazilian banks still write OFX in Latin-1; a broken UTF-8 read shows U+FFFD.
      if (text.includes("�")) text = new TextDecoder("windows-1252").decode(await file.arrayBuffer());
      const { lines, balance } = parseStatement(file.name, text);
      if (lines.length === 0) return toast.error(t("fin.imp.no_lines"));
      let inserted = 0;
      for (let i = 0; i < lines.length; i += BATCH) {
        const last = i + BATCH >= lines.length;
        inserted += (await finance.importLines(accountId, lines.slice(i, i + BATCH), last ? balance : undefined)).inserted;
      }
      toast.success(fill(t("fin.imp.done"), { n: inserted, t: lines.length }));
      await client.invalidateQueries({ queryKey: ["finance"] });
    } catch (err) {
      toast.error(err instanceof Error && err.message === "csv_columns" ? t("fin.imp.csv_columns") : errorMessage(err, "common.save_failed", t));
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  };
  return (
    <>
      <input ref={input} type="file" accept=".ofx,.csv,text/csv" className="sr-only" tabIndex={-1} aria-hidden="true"
        onChange={(e) => { const f = e.target.files?.[0]; if (f) void pick(f); }} />
      <Button size="sm" variant="outline" disabled={busy} onClick={() => input.current?.click()}>
        <FileUp className="size-4" />{busy ? t("fin.imp.reading") : t("fin.imp.file")}
      </Button>
    </>
  );
}

/** Creates an account that takes imported files, for a bank the connector cannot reach. */
export function ManualAccountDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [institution, setInstitution] = useState("");
  const [name, setName] = useState("");
  const [type, setType] = useState<"BANK" | "CREDIT">("CREDIT");
  const valid = institution.trim() !== "" && name.trim() !== "";
  const create = useMutation({
    mutationFn: () => finance.createManualAccount({ institution: institution.trim(), name: name.trim(), type }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.imp.created"));
      setInstitution("");
      setName("");
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });
  const types = { CREDIT: t("fin.acc.CREDIT_CARD"), BANK: t("fin.acc.CHECKING_ACCOUNT") };
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("fin.imp.new")}</DialogTitle>
          <DialogDescription>{t("fin.imp.help")}</DialogDescription>
        </DialogHeader>
        <form id="manual-form" className="grid gap-3" onSubmit={(e) => { e.preventDefault(); if (valid) create.mutate(); }}>
          <div className="grid gap-1.5">
            <Label htmlFor="manual-inst">{t("fin.imp.institution")}</Label>
            <Input id="manual-inst" maxLength={60} value={institution} onChange={(e) => setInstitution(e.target.value)} placeholder="Nubank" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="manual-name">{t("fin.imp.name")}</Label>
              <Input id="manual-name" maxLength={60} value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label>{t("fin.imp.type")}</Label>
              <Select value={type} items={types} onValueChange={(v) => setType(v === "BANK" ? "BANK" : "CREDIT")}>
                <SelectTrigger aria-label={t("fin.imp.type")}><SelectValue /></SelectTrigger>
                <SelectContent>{Object.entries(types).map(([value, l]) => <SelectItem key={value} value={value}>{l}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
        </form>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button type="submit" form="manual-form" disabled={!valid || create.isPending}>{t("fin.imp.create")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
