// Personal finance API (/v1/finance). Money is integer cents; a negative amount leaves the user.
import { apiFetch, apiStream } from "@/lib/api/client";
import type { ApiErrorBody, Paged } from "@/lib/api/types";

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
  /** CHECKING_ACCOUNT, SAVINGS_ACCOUNT, CREDIT_CARD... */
  subtype: string;
  name: string;
  number: string;
  /** On a card, the open bill. */
  balance: Cents;
  credit_limit: Cents | null;
  available: Cents | null;
  bill_close_date: string | null;
  bill_due_date: string | null;
  /** bill_due_date, or the same day of a later month once it passed. */
  next_bill_due: string | null;
  last_bill_due: string | null;
  last_bill: Cents | null;
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
  /** Imported statements: no bank behind it, nothing to sync. */
  manual: boolean;
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
  /** The original transaction when this row is a split part. */
  split_of: string;
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
  recurring: Recurring[];
  installments: { description: string; amount: Cents; remaining: number }[];
  /** Only for the current month. */
  forecast: { days_left: number; pending_bills: Cents; expected_income: Cents; projected: Cents } | null;
  investments: Investment[];
}

export interface Investment {
  id: string;
  connection_id: string;
  name: string;
  type: string;
  balance: Cents;
}

export interface YearReport {
  year: number;
  income: Cents;
  expense: Cents;
  categories: CategoryTotal[];
  months: MonthTotal[];
}

/** A charge seen in several of the last months. Amounts are negative. */
export interface Recurring {
  /** Send to ignoreRecurring to dismiss it. */
  key: string;
  name: string;
  amount: Cents;
  months: number;
  last_date: string;
  changed: boolean;
  previous: Cents;
  last: Cents;
}

/** A subscription or fixed bill the user registered. Amounts are positive. */
export interface Subscription {
  id: string;
  name: string;
  amount: Cents;
  day: number;
  /** 0 every month; 1-12 once a year in that month. */
  month: number;
  keyword: string;
  category_id: string;
  account_id: string;
  last_paid: string;
  last_amount: Cents;
  paid: boolean;
  late: boolean;
  next_due: string;
}

export type SubscriptionInput = Pick<Subscription, "name" | "amount" | "day" | "month" | "keyword" | "category_id" | "account_id">;

export interface CalendarEvent {
  date: string;
  kind: "bill" | "recurring" | "subscription" | "income";
  name: string;
  amount: Cents;
  /** Bank balance after this event. */
  balance: Cents;
}

export interface Calendar {
  start: Cents;
  lowest: Cents;
  events: CalendarEvent[];
}

export interface Goal {
  id: string;
  name: string;
  target: Cents;
  /** Typed by the user; ignored when an account is linked. */
  saved: Cents;
  account_id: string;
  due_date: string;
  current: Cents;
}

/** One statement line, parsed in the browser from an OFX or CSV file. */
export interface ImportLine {
  id?: string;
  date: string;
  description: string;
  amount: Cents;
}

export interface MonthTotal {
  month: string;
  income: Cents;
  expense: Cents;
}

export interface Person {
  name: string;
  /** Matches transfers received from this person; empty when none came in. */
  keyword: string;
  sent: Cents;
  received: Cents;
  refunded: Cents;
  count: number;
  last_date: string;
}

export interface RuleSuggestion {
  keyword: string;
  example: string;
  count: number;
  total: Cents;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

/** A change the assistant proposes; nothing happens until the user applies it. */
export interface ChatAction {
  kind: "rule" | "subscription";
  keyword: string;
  category_id?: string;
  category_name?: string;
  name?: string;
  amount?: Cents;
  day?: number;
}

export type ChatEvent =
  | { type: "delta" | "status"; text: string }
  | { type: "reset" }
  | { type: "done"; answer: string; actions?: ChatAction[] }
  | { type: "error"; error: ApiErrorBody };

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
  trend: (months = 12) => apiFetch<{ items: MonthTotal[] }>(`${base}/trend${query({ months })}`),
  people: () => apiFetch<{ items: Person[] }>(`${base}/people`),
  recategorize: () => apiFetch<{ changed: number }>(`${base}/recategorize`, { method: "POST" }),
  transactions: (p: TransactionQuery, signal?: AbortSignal) => apiFetch<Paged<Transaction>>(`${base}/transactions${query(p)}`, { signal }),
  setCategory: (transactionId: string, categoryId: string) =>
    apiFetch<{ keyword: string }>(`${base}/transactions/${id(transactionId)}`, { method: "PATCH", json: { category_id: categoryId } }),

  categories: () => apiFetch<{ items: Category[] }>(`${base}/categories`),
  createCategory: (c: Omit<Category, "id">) => apiFetch<Category>(`${base}/categories`, { method: "POST", json: c }),
  updateCategory: (categoryId: string, patch: { name?: string; limit?: Cents }) =>
    apiFetch<Category>(`${base}/categories/${id(categoryId)}`, { method: "PATCH", json: patch }),
  removeCategory: (categoryId: string) => apiFetch<void>(`${base}/categories/${id(categoryId)}`, { method: "DELETE" }),

  rules: () => apiFetch<{ items: Rule[] }>(`${base}/rules`),
  ruleSuggestions: () => apiFetch<{ items: RuleSuggestion[] }>(`${base}/rules/suggestions`),
  previewRule: (keyword: string) => apiFetch<{ matches: number }>(`${base}/rules/preview${query({ keyword })}`),
  createRule: (keyword: string, categoryId: string) =>
    apiFetch<{ rule: Rule; applied: number }>(`${base}/rules`, { method: "POST", json: { keyword, category_id: categoryId } }),
  removeRule: (ruleId: string) => apiFetch<void>(`${base}/rules/${id(ruleId)}`, { method: "DELETE" }),

  split: (transactionId: string, amount: Cents, categoryId: string) =>
    apiFetch<void>(`${base}/transactions/${id(transactionId)}/split`, { method: "POST", json: { amount, category_id: categoryId } }),
  unsplit: (transactionId: string) => apiFetch<void>(`${base}/transactions/${id(transactionId)}/split`, { method: "DELETE" }),
  year: (year: number) => apiFetch<YearReport>(`${base}/year${query({ year })}`),
  calendar: () => apiFetch<Calendar>(`${base}/calendar`),
  createManualAccount: (a: { institution: string; name: string; type: "BANK" | "CREDIT" }) =>
    apiFetch<Account>(`${base}/accounts`, { method: "POST", json: a }),
  importLines: (accountId: string, transactions: ImportLine[], balance?: Cents) =>
    apiFetch<{ inserted: number }>(`${base}/accounts/${id(accountId)}/import`, { method: "POST", json: { transactions, balance } }),
  subscriptions: () => apiFetch<{ items: Subscription[]; detected: Recurring[]; monthly: Cents }>(`${base}/subscriptions`),
  createSubscription: (s: SubscriptionInput) => apiFetch<Subscription>(`${base}/subscriptions`, { method: "POST", json: s }),
  updateSubscription: (subId: string, s: SubscriptionInput) => apiFetch<Subscription>(`${base}/subscriptions/${id(subId)}`, { method: "PUT", json: s }),
  removeSubscription: (subId: string) => apiFetch<void>(`${base}/subscriptions/${id(subId)}`, { method: "DELETE" }),
  ignoreRecurring: (key: string) => apiFetch<void>(`${base}/subscriptions/ignore`, { method: "POST", json: { key } }),
  goals: () => apiFetch<{ items: Goal[] }>(`${base}/goals`),
  createGoal: (g: Omit<Goal, "id" | "current">) => apiFetch<Goal>(`${base}/goals`, { method: "POST", json: g }),
  updateGoal: (goalId: string, g: Omit<Goal, "id" | "current">) => apiFetch<Goal>(`${base}/goals/${id(goalId)}`, { method: "PUT", json: g }),
  removeGoal: (goalId: string) => apiFetch<void>(`${base}/goals/${id(goalId)}`, { method: "DELETE" }),

  chat: (messages: ChatMessage[], onEvent: (e: ChatEvent) => void, signal?: AbortSignal) =>
    apiStream<ChatEvent>(`${base}/assistant/chat`, { method: "POST", json: { messages }, signal }, onEvent),
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
