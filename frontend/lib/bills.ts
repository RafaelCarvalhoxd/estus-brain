import "server-only";

// Mirrors backend/internal/httpapi/dto_bills.go by hand, same convention as
// lib/api.ts — kept as its own file so this module never needs to touch the
// shared types.ts / api.ts files other work is editing in parallel.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Money {
  cents: number;
  formatted: string;
}

export type BillDirection = "pagar" | "receber";
export type BillStatus = "pendente" | "atrasado" | "pago" | "recebido";

export interface Bill {
  id: string;
  description: string;
  amount: Money;
  due_date: string;
  direction: BillDirection;
  category_id?: string;
  paid_at?: string;
  recurring: boolean;
  status: BillStatus;
}

export interface BillSummary {
  payable_open: Money;
  receivable_open: Money;
  overdue_count: number;
}

export interface CreateBillInput {
  description: string;
  amount_cents: number;
  due_date: string;
  direction: BillDirection;
  category_id?: string;
  recurring?: boolean;
}

async function billsFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api ${path} -> ${res.status}: ${body}`);
  }
  return res.json() as Promise<T>;
}

export function listBills(direction?: BillDirection): Promise<Bill[]> {
  const query = direction ? `?direction=${direction}` : "";
  return billsFetch<Bill[]>(`/api/bills${query}`, { cache: "no-store" });
}

export function getBillSummary(): Promise<BillSummary> {
  return billsFetch<BillSummary>("/api/bills/summary", { cache: "no-store" });
}

// getBillsReceived is the closest thing this app has to "entradas": the
// total of receivable bills actually marked received within that month.
export function getBillsReceived(yearMonth: string): Promise<Money> {
  return billsFetch<{ received: Money }>(`/api/bills/received?month=${yearMonth}`, { cache: "no-store" }).then(
    (r) => r.received,
  );
}

export function createBill(input: CreateBillInput): Promise<Bill> {
  return billsFetch<Bill>("/api/bills", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function markBillPaid(id: string): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}/paid`, { method: "POST" });
}

export function updateBill(id: string, input: CreateBillInput): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteBill(id: string): Promise<void> {
  return billsFetch<void>(`/api/bills/${id}`, { method: "DELETE" });
}
