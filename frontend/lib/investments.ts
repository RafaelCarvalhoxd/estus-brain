import "server-only";

// Mirrors backend/internal/httpapi/dto_investments.go by hand.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Money {
  cents: number;
  formatted: string;
}

export type InvestmentKind = "cdb" | "tesouro" | "lci_lca" | "poupanca" | "fundo" | "acoes" | "fii" | "cripto" | "outro";
export type RatePeriod = "mes" | "ano";

export interface Contribution {
  id: string;
  amount: Money;
  date: string;
  /** The opening amount: already invested, so it never left the balance. */
  initial: boolean;
}

export interface Investment {
  id: string;
  name: string;
  kind: InvestmentKind;
  /** Hundredths of a percentage point: 1% = 100. */
  rate_bp: number;
  rate_period: RatePeriod;
  invested: Money;
  /** Every contribution compounded at the rate up to today. */
  estimated: Money;
  contributions: Contribution[];
}

export interface InvestmentList {
  investments: Investment[];
  invested: Money;
  estimated: Money;
  as_of: string;
}

export interface InvestmentInput {
  name: string;
  kind: InvestmentKind;
  rate_bp: number;
  rate_period: RatePeriod;
}

async function investmentFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api ${path} -> ${res.status}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function listInvestments(): Promise<InvestmentList> {
  return investmentFetch<InvestmentList>("/api/investments", { cache: "no-store" });
}

export function createInvestment(input: InvestmentInput & { amount_cents: number; date: string }): Promise<Investment> {
  return investmentFetch<Investment>("/api/investments", { method: "POST", body: JSON.stringify(input) });
}

export function updateInvestment(id: string, input: InvestmentInput): Promise<void> {
  return investmentFetch<void>(`/api/investments/${id}`, { method: "PUT", body: JSON.stringify(input) });
}

export function deleteInvestment(id: string): Promise<void> {
  return investmentFetch<void>(`/api/investments/${id}`, { method: "DELETE" });
}

export function addContribution(id: string, amountCents: number, date: string): Promise<Contribution> {
  return investmentFetch<Contribution>(`/api/investments/${id}/contributions`, {
    method: "POST",
    body: JSON.stringify({ amount_cents: amountCents, date }),
  });
}

export function deleteContribution(id: string, contributionId: string): Promise<void> {
  return investmentFetch<void>(`/api/investments/${id}/contributions/${contributionId}`, { method: "DELETE" });
}
