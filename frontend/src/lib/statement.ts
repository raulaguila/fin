// Bank statement files parsed in the browser: OFX and the CSVs banks export
// (Nubank card and account, and any file with date, description and amount
// columns). The server only receives the lines, in cents, negative = money out.
import type { Cents, ImportLine } from "@/lib/api/finance";

export interface Statement {
  lines: ImportLine[];
  /** Ledger balance the OFX file states, when present. */
  balance?: Cents;
}

/** "1.234,56", "1234.56", "-12,5", "R$ 10,00" to cents; null when not a number. */
export function amountToCents(text: string): number | null {
  let s = text.trim().replace(/[R$\s ]/g, "");
  if (s === "") return null;
  const negative = s.startsWith("-") || (s.startsWith("(") && s.endsWith(")"));
  s = s.replace(/^[-+(]|\)$/g, "");
  // The last separator is the decimal one; the other is grouping.
  const lastComma = s.lastIndexOf(","), lastDot = s.lastIndexOf(".");
  if (lastComma > lastDot) s = s.replace(/\./g, "").replace(",", ".");
  else s = s.replace(/,/g, "");
  if (!/^\d+(\.\d+)?$/.test(s)) return null;
  const cents = Math.round(Number(s) * 100);
  return negative ? -cents : cents;
}

/** YYYY-MM-DD, DD/MM/YYYY or YYYYMMDD (OFX) to YYYY-MM-DD; null otherwise. */
export function toISODate(text: string): string | null {
  const s = text.trim();
  let m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s) ?? /^(\d{4})(\d{2})(\d{2})/.exec(s);
  if (m) return `${m[1]}-${m[2]}-${m[3]}`;
  m = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(s);
  return m ? `${m[3]}-${m[2]}-${m[1]}` : null;
}

function tag(block: string, name: string): string {
  // SGML OFX leaves tags open; XML OFX closes them. Both end the value at the next tag or line.
  const m = new RegExp(`<${name}>([^<\\r\\n]*)`, "i").exec(block);
  return m ? m[1].trim() : "";
}

export function parseOFX(text: string): Statement {
  const lines: ImportLine[] = [];
  for (const block of text.split(/<STMTTRN>/i).slice(1)) {
    const date = toISODate(tag(block, "DTPOSTED"));
    const amount = amountToCents(tag(block, "TRNAMT"));
    const description = tag(block, "MEMO") || tag(block, "NAME");
    if (!date || amount === null || amount === 0 || !description) continue;
    lines.push({ id: tag(block, "FITID") || undefined, date, description, amount });
  }
  const ledger = /<LEDGERBAL>([\s\S]*?)(<\/LEDGERBAL>|<AVAILBAL>|$)/i.exec(text);
  const balance = ledger ? amountToCents(tag(ledger[1], "BALAMT")) : null;
  return balance === null ? { lines } : { lines, balance };
}

/** Splits one CSV row, honoring quotes. */
function cells(row: string, sep: string): string[] {
  const out: string[] = [];
  let cur = "", quoted = false;
  for (let i = 0; i < row.length; i++) {
    const ch = row[i];
    if (ch === '"') {
      if (quoted && row[i + 1] === '"') { cur += '"'; i++; } else quoted = !quoted;
    } else if (ch === sep && !quoted) { out.push(cur.trim()); cur = ""; } else cur += ch;
  }
  out.push(cur.trim());
  return out;
}

const norm = (s: string) => s.toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, "");
const find = (header: string[], names: string[]) => header.findIndex((h) => names.includes(norm(h)));

export function parseCSV(text: string): Statement {
  const rows = text.replace(/^﻿/, "").split(/\r?\n/).filter((r) => r.trim() !== "");
  if (rows.length < 2) return { lines: [] };
  const sep = (rows[0].match(/;/g)?.length ?? 0) > (rows[0].match(/,/g)?.length ?? 0) ? ";" : ",";
  const header = cells(rows[0], sep);
  const di = find(header, ["date", "data", "data lancamento", "data da transacao"]);
  const ai = find(header, ["amount", "valor", "valor (r$)"]);
  const ti = find(header, ["description", "descricao", "title", "historico", "lancamento", "estabelecimento"]);
  const ii = find(header, ["id", "identificador"]);
  if (di < 0 || ai < 0 || ti < 0) throw new Error("csv_columns");
  // The Nubank card file (date,title,amount) lists a charge as a positive amount.
  const flip = norm(header[ti]) === "title" ? -1 : 1;
  const lines: ImportLine[] = [];
  for (const row of rows.slice(1)) {
    const c = cells(row, sep);
    const date = toISODate(c[di] ?? "");
    const amount = amountToCents(c[ai] ?? "");
    const description = (c[ti] ?? "").trim();
    if (!date || amount === null || amount === 0 || !description) continue;
    lines.push({ id: ii >= 0 && c[ii] ? c[ii] : undefined, date, description, amount: amount * flip });
  }
  return { lines };
}

export function parseStatement(name: string, text: string): Statement {
  return /\.ofx$/i.test(name) || /<OFX>/i.test(text) ? parseOFX(text) : parseCSV(text);
}
