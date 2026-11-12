// Personal finance API (/v1/finance). Money is integer cents; a negative amount leaves the user.
import { apiFetch } from "@/lib/api/client";
import type { Paged } from "@/lib/api/types";

export type Cents = number;
export type CategoryKind = "expense" | "income" | "transfer";
export type ConnectionStatus = "syncing" | "ok" | "error" | "login_required";

export interface FinanceStatus {
  open_finance: boolean;
  assistant: boolean;
}

export interface Account {
  id: string;
  connection_id: string;
  type: "BANK" | "CREDIT";
  name: string;
  number: string;
  /** On a card, the open bill. */
  balance: Cents;
  credit_limit: Cents | null;
  available: Cents | null;
  bill_close_date: string | null;
  bill_due_date: string | null;
}

export interface Connection {
  id: string;
  item_id: string;
  institution: string;
  image_url: string;
  status: ConnectionStatus;
  error: string;
  consent_expires_at: string | null;
  last_sync_at: string | null;
  created_at: string;
  accounts: Account[];
}

export interface Transaction {
  id: string;
  account_id: string;
  account_name: string;
  account_type: "BANK" | "CREDIT";
  institution: string;
  date: string;
  description: string;
  amount: Cents;
  status: string;
  category_id: string;
  category_name: string;
  category_kind: CategoryKind | "";
  category_source: "auto" | "rule" | "user";
  installment: number;
  installments: number;
}

export interface Category {
  id: string;
  name: string;
  kind: CategoryKind;
  limit: Cents;
}

export interface Rule {
  id: string;
  keyword: string;
  category_id: string;
  category_name: string;
  hits: number;
}

export interface CategoryTotal {
  /** Empty: uncategorized. */
  category_id: string;
  name: string;
  kind: CategoryKind;
  limit: Cents;
  total: Cents;
  previous: Cents;
  count: number;
}

export interface Summary {
  month: string;
  income: Cents;
  expense: Cents;
  categories: CategoryTotal[];
  accounts: Account[];
  institutions: Record<string, string>;
  recent: Transaction[];
  recurring: { name: string; amount: Cents; months: number; last_date: string; changed: boolean; previous: Cents }[];
  installments: { description: string; amount: Cents; remaining: number }[];
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface TransactionQuery {
  page: number;
  limit: number;
  from?: string;
  to?: string;
  search?: string;
  account_id?: string;
  category_id?: string;
}

function query(params: object): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") q.set(k, String(v));
  const s = q.toString();
  return s ? `?${s}` : "";
}

const id = (v: string) => encodeURIComponent(v);
const base = "/v1/finance";

export const finance = {
  status: () => apiFetch<FinanceStatus>(`${base}/status`),
  connectToken: (connectionId?: string) =>
    apiFetch<{ access_token: string }>(`${base}/connect-token`, { method: "POST", json: connectionId ? { connection_id: connectionId } : {} }),
  connections: () => apiFetch<{ items: Connection[] }>(`${base}/connections`),
  addConnection: (itemId: string) => apiFetch<Connection>(`${base}/connections`, { method: "POST", json: { item_id: itemId } }),
  sync: (connectionId: string) => apiFetch<void>(`${base}/connections/${id(connectionId)}/sync`, { method: "POST" }),
  removeConnection: (connectionId: string) => apiFetch<void>(`${base}/connections/${id(connectionId)}`, { method: "DELETE" }),

  summary: (month?: string) => apiFetch<Summary>(`${base}/summary${query({ month })}`),
  transactions: (p: TransactionQuery, signal?: AbortSignal) => apiFetch<Paged<Transaction>>(`${base}/transactions${query(p)}`, { signal }),
  setCategory: (transactionId: string, categoryId: string) =>
    apiFetch<void>(`${base}/transactions/${id(transactionId)}`, { method: "PATCH", json: { category_id: categoryId } }),

  categories: () => apiFetch<{ items: Category[] }>(`${base}/categories`),
  createCategory: (c: Omit<Category, "id">) => apiFetch<Category>(`${base}/categories`, { method: "POST", json: c }),
  updateCategory: (categoryId: string, patch: { name?: string; limit?: Cents }) =>
    apiFetch<Category>(`${base}/categories/${id(categoryId)}`, { method: "PATCH", json: patch }),
  removeCategory: (categoryId: string) => apiFetch<void>(`${base}/categories/${id(categoryId)}`, { method: "DELETE" }),

  rules: () => apiFetch<{ items: Rule[] }>(`${base}/rules`),
  previewRule: (keyword: string) => apiFetch<{ matches: number }>(`${base}/rules/preview${query({ keyword })}`),
  createRule: (keyword: string, categoryId: string) =>
    apiFetch<{ rule: Rule; applied: number }>(`${base}/rules`, { method: "POST", json: { keyword, category_id: categoryId } }),
  removeRule: (ruleId: string) => apiFetch<void>(`${base}/rules/${id(ruleId)}`, { method: "DELETE" }),

  chat: (messages: ChatMessage[]) => apiFetch<{ answer: string }>(`${base}/assistant/chat`, { method: "POST", json: { messages } }),
};

/** Month as YYYY-MM of a date, in local time. */
export function monthKey(d = new Date()): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/** First day of month m and of the month after, as YYYY-MM-DD. */
export function monthRange(m: string): { from: string; to: string } {
  const [y, mo] = m.split("-").map(Number);
  const next = mo === 12 ? `${y + 1}-01` : `${y}-${String(mo + 1).padStart(2, "0")}`;
  return { from: `${m}-01`, to: `${next}-01` };
}

export function shiftMonth(m: string, delta: number): string {
  const [y, mo] = m.split("-").map(Number);
  return monthKey(new Date(y, mo - 1 + delta, 1));
}
