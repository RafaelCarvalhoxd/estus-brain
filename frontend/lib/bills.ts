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
export type BillPaymentMethod = "debito" | "credito" | "pix";

export interface Bill {
  id: string;
  description: string;
  amount: Money;
  due_date: string;
  direction: BillDirection;
  category_id?: string;
  paid_at?: string;
  recurring: boolean;
  // amount_estimated is per OCCURRENCE: this bill's own amount was carried
  // over and not yet confirmed. Paying or editing it clears this — it says
  // nothing about whether the series itself varies (see amount_varies).
  amount_estimated: boolean;
  // amount_varies is per SERIES: whether the amount is expected to change
  // every month. It survives this occurrence being paid or corrected, and is
  // what seeds the next occurrence's own amount_estimated.
  amount_varies: boolean;
  // series_ended means the owner used "Encerrar repetição": Materialize will
  // not grow this series any further, though every occurrence already born
  // stays exactly as it is.
  series_ended: boolean;
  payment_method?: BillPaymentMethod;
  credit_card_id?: string;
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
  amount_estimated?: boolean;
  amount_varies?: boolean;
  payment_method?: BillPaymentMethod;
  credit_card_id?: string;
}

// PayBillInput is what the owner confirms when settling a payable bill: the
// amount that actually left the account, not the bill's own (possibly
// estimated) amount. Mirrors backend/internal/httpapi/dto_bills.go's
// payBillRequest.
export interface PayBillInput {
  paid_on: string;
  amount_cents: number;
  category_id: string;
  payment_method: BillPaymentMethod;
  credit_card_id?: string;
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

// listBills lists bills, optionally filtered by direction and scoped to a
// single month (?month=YYYY-MM) — how the Contas screen reads a month now
// that it has navigation. The backend materializes that month's recurring
// series before listing it, so a bare fetch is enough; a month-less call
// (used by the home dashboard's "upcoming" widget) still lists every open
// bill regardless of when it's due.
export function listBills(direction?: BillDirection, month?: string): Promise<Bill[]> {
  const params = new URLSearchParams();
  if (direction) params.set("direction", direction);
  if (month) params.set("month", month);
  const query = params.toString();
  return billsFetch<Bill[]>(`/api/bills${query ? `?${query}` : ""}`, { cache: "no-store" });
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

// payBill settles a payable bill and records its expense in the same
// backend commit (POST /api/bills/{id}/pay) — the receivable path above
// (markBillPaid) stays untouched, since a receivable never records an
// expense. unpayBill reverses it (DELETE /api/bills/{id}/paid): the bill
// goes back to pending and the expense it created is deleted, also in one
// commit.
export function payBill(id: string, input: PayBillInput): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}/pay`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function unpayBill(id: string): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}/paid`, { method: "DELETE" });
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

// endSeries stops a recurring bill's series from growing new occurrences
// (Materialize skips it), without touching any occurrence already born.
// resumeSeries undoes it, so the next month opened picks the series back up.
export function endSeries(id: string): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}/end-series`, { method: "POST" });
}

export function resumeSeries(id: string): Promise<Bill> {
  return billsFetch<Bill>(`/api/bills/${id}/end-series`, { method: "DELETE" });
}
