// Mirrors the JSON DTOs in backend/internal/httpapi/dto.go by hand. The
// backend is the source of truth; if you add or rename a field there,
// update it here too. A generated client (from an OpenAPI spec) is a
// reasonable next step once the API stops changing shape every day — not
// worth the tooling yet for four endpoints.

export interface Money {
  cents: number;
  formatted: string;
}

export interface Category {
  id: string;
  name: string;
  nature: "essencial" | "variavel" | "investimento";
  color: string;
}

export interface CreditCard {
  id: string;
  name: string;
  closing_day: number;
  due_day: number;
}

export type PaymentMethod = "debito" | "credito" | "pix";

export interface Transaction {
  id: string;
  description: string;
  amount: Money;
  category_id: string;
  category_name: string;
  category_color: string;
  payment_method: PaymentMethod;
  purchase_date: string;
  competence_month: string;
  is_recurring: boolean;
  installment_number?: number;
  installment_total?: number;
}

export interface CategorySlice {
  category_id: string;
  name: string;
  color: string;
  total: Money;
}

export interface CategoryComparison {
  category_id: string;
  name: string;
  color: string;
  current: Money;
  previous: Money;
}

export interface WeekBucket {
  label: string;
  total: Money;
}

export interface MonthSummary {
  month: string;
  total: Money;
  previous_month: Money;
  recurring: Money;
  open_installments: Money;
  open_invoice: Money;
  categories: CategorySlice[];
  comparison: CategoryComparison[];
  weeks: WeekBucket[];
  transactions: Transaction[];
}

export interface CreateTransactionInput {
  description: string;
  amount_cents: number;
  category_id: string;
  payment_method: PaymentMethod;
  purchase_date: string;
  credit_card_id?: string;
  installments?: number;
  is_recurring?: boolean;
}
