// The backend takes an archive in parts: its body limit is small. Settings go
// first (categories before the rules and transactions that name them), then
// each account's transactions and the bank overrides, PART rows at a time.
// Sending a part twice adds nothing twice, so a failed import can just run again.

export const PART = 500;

type Rows = Record<string, unknown>[];

export interface ArchiveFile {
  version: number;
  accounts?: (Record<string, unknown> & { transactions?: Rows })[];
  overrides?: Rows;
  [key: string]: unknown;
}

function chunks<T>(list: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < list.length; i += size) out.push(list.slice(i, i + size));
  return out;
}

/** Throws when the file is not a Fin archive. */
export function archiveParts(file: unknown): Record<string, unknown>[] {
  if (!file || typeof file !== "object" || typeof (file as ArchiveFile).version !== "number") throw new Error("not a Fin archive");
  const { accounts = [], overrides = [], ...settings } = file as ArchiveFile;
  const version = settings.version;
  const parts: Record<string, unknown>[] = [settings];
  for (const { transactions = [], ...account } of accounts) {
    // An account with no transactions still goes once, so it is created.
    for (const rows of transactions.length ? chunks(transactions, PART) : [[]]) parts.push({ version, accounts: [{ ...account, transactions: rows }] });
  }
  for (const rows of chunks(overrides, PART)) parts.push({ version, overrides: rows });
  return parts;
}
