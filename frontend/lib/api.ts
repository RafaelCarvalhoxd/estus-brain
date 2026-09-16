import "server-only";
import type {
  Category,
  CreditCard,
  MonthSummary,
  CreateTransactionInput,
} from "./types";

// API_URL is only ever read on the server: this module is marked
// server-only so a client component that accidentally imports it fails the
// build instead of leaking the backend's internal address into the
// browser bundle. The browser never talks to the Go API directly — every
// request goes through a Next.js server component or server action, which
// is what lets the Go service live entirely behind the mTLS edge with no
// public listener of its own.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
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

export function listCategories(): Promise<Category[]> {
  return apiFetch<Category[]>("/api/categories", { cache: "no-store" });
}

export function listCreditCards(): Promise<CreditCard[]> {
  return apiFetch<CreditCard[]>("/api/credit-cards", { cache: "no-store" });
}

export function getMonthSummary(yearMonth: string): Promise<MonthSummary> {
  return apiFetch<MonthSummary>(`/api/months/${yearMonth}`, { cache: "no-store" });
}

export function createTransaction(input: CreateTransactionInput): Promise<unknown> {
  return apiFetch("/api/transactions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateTransaction(id: string, description: string, categoryId: string): Promise<unknown> {
  return apiFetch(`/api/transactions/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ description, category_id: categoryId }),
  });
}

export function deleteTransaction(id: string): Promise<unknown> {
  return apiFetch(`/api/transactions/${id}`, { method: "DELETE" });
}

export function updateCategoryBudget(id: string, monthlyBudgetCents: number | null): Promise<Category> {
  return apiFetch<Category>(`/api/categories/${id}/budget`, {
    method: "PATCH",
    body: JSON.stringify({ monthly_budget_cents: monthlyBudgetCents }),
  });
}

export interface CategoryInput {
  name: string;
  nature: Category["nature"];
  color: string;
}

export function createCategory(input: CategoryInput): Promise<Category> {
  return apiFetch<Category>("/api/categories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCategory(id: string, input: CategoryInput): Promise<Category> {
  return apiFetch<Category>(`/api/categories/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteCategory(id: string): Promise<unknown> {
  return apiFetch(`/api/categories/${id}`, { method: "DELETE" });
}

export interface CreditCardInput {
  name: string;
  closing_day: number;
  due_day: number;
}

export function createCreditCard(input: CreditCardInput): Promise<CreditCard> {
  return apiFetch<CreditCard>("/api/credit-cards", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCreditCard(id: string, input: CreditCardInput): Promise<CreditCard> {
  return apiFetch<CreditCard>(`/api/credit-cards/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteCreditCard(id: string): Promise<unknown> {
  return apiFetch(`/api/credit-cards/${id}`, { method: "DELETE" });
}
