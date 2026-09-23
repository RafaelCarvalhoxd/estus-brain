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
  /** despesa counts as spending on the dashboard, receita as income. */
  kind: "despesa" | "receita";
  color: string;
  monthly_budget?: Money;
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
  category_kind: "despesa" | "receita";
  payment_method: PaymentMethod;
  purchase_date: string;
  competence_month: string;
  /** YYYY-MM of the card invoice a credit purchase is billed on. */
  invoice_month?: string;
  credit_card_id?: string;
  /** The whole purchase: all installments together. */
  purchase_total: Money;
  is_recurring: boolean;
  installment_number?: number;
  installment_total?: number;
}

export interface CategorySlice {
  category_id: string;
  name: string;
  color: string;
  total: Money;
  monthly_budget?: Money;
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
  variable: Money;
  /** What actually left the account in the month: débito, pix and card
   * invoices paid. */
  paid_out: Money;
  /** Recurring bills due this month and not paid yet — fixed spending to come. */
  open_fixed: Money;
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

export interface CardInvoice {
  month: string; // YYYY-MM
  due_date: string;
  total: Money;
  purchases: number;
  paid: boolean;
}

// Mirrors backend/internal/httpapi/handlers_card_spending.go.
export interface CardOverview {
  card: CreditCard;
  /** The invoice a purchase made today lands on. */
  current: CardInvoice;
  months: CardInvoice[];
  categories: { name: string; color: string; total: Money }[];
}
