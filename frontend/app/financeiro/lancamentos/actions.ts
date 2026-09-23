"use server";

import { revalidatePath } from "next/cache";
import { createTransaction, updateTransaction, deleteTransaction, replaceTransaction } from "@/lib/api";
import type { CreateTransactionInput, PaymentMethod } from "@/lib/types";

export type CreateTransactionState = {
  status: "idle" | "error" | "success";
  message?: string;
};

// parseAmount accepts what a Brazilian keyboard actually types — "189,90"
// or "1.234,56" — rather than forcing the user to think in dots and cents.
function parseAmountToCents(raw: string): number | null {
  const normalized = raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", ".");
  const value = Number.parseFloat(normalized);
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}

// readTransactionForm turns the "Novo lançamento" form into the API input,
// or an error message for the first field that is wrong.
function readTransactionForm(formData: FormData): { input: CreateTransactionInput } | { error: string } {
  const description = String(formData.get("description") ?? "").trim();
  const typedCents = parseAmountToCents(String(formData.get("amount") ?? ""));
  const categoryId = String(formData.get("category_id") ?? "");
  const paymentMethod = String(formData.get("payment_method") ?? "") as PaymentMethod;
  const purchaseDate = String(formData.get("purchase_date") ?? "");
  const creditCardId = String(formData.get("credit_card_id") ?? "") || undefined;
  const installments = Number(formData.get("installments") ?? 1);
  const isRecurring = formData.get("is_recurring") === "1";
  // "parcela" means the amount typed is one installment, not the purchase.
  const perInstallment = formData.get("amount_mode") === "parcela" && paymentMethod === "credito" && installments > 1;
  const amountCents = typedCents === null ? null : perInstallment ? typedCents * installments : typedCents;

  if (!description) return { error: "Descrição é obrigatória." };
  if (amountCents === null) return { error: "Valor inválido." };
  if (!categoryId) return { error: "Selecione uma categoria." };
  if (!purchaseDate) return { error: "Selecione a data da compra." };
  if (paymentMethod === "credito" && !creditCardId) return { error: "Selecione o cartão." };

  return {
    input: {
      description,
      amount_cents: amountCents,
      category_id: categoryId,
      payment_method: paymentMethod,
      purchase_date: purchaseDate,
      credit_card_id: paymentMethod === "credito" ? creditCardId : undefined,
      installments: paymentMethod === "credito" ? installments : 1,
      is_recurring: isRecurring,
    },
  };
}

export async function createTransactionAction(
  _prevState: CreateTransactionState,
  formData: FormData,
): Promise<CreateTransactionState> {
  const read = readTransactionForm(formData);
  if ("error" in read) return { status: "error", message: read.error };
  try {
    await createTransaction(read.input);
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateFinance();
  return { status: "success", message: "Lançamento salvo." };
}

// replaceTransactionAction rewrites the whole purchase id belongs to —
// every installment — from the same form.
export async function replaceTransactionAction(
  id: string,
  _prevState: CreateTransactionState,
  formData: FormData,
): Promise<CreateTransactionState> {
  const read = readTransactionForm(formData);
  if ("error" in read) return { status: "error", message: read.error };
  try {
    await replaceTransaction(id, read.input);
  } catch (err) {
    if (isConflict(err)) {
      return { status: "error", message: "Este lançamento veio do pagamento de uma conta e não pode virar parcelado." };
    }
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateFinance();
  revalidatePath("/financeiro/contas");
  revalidatePath("/financeiro/cartoes");
  return { status: "success", message: "Lançamento atualizado." };
}

function revalidateFinance() {
  revalidatePath("/");
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/lancamentos");
  revalidatePath("/financeiro/categorias");
}

export async function updateTransactionAction(id: string, description: string, categoryId: string): Promise<void> {
  await updateTransaction(id, description, categoryId);
  revalidateFinance();
}

// isConflict recognizes the 409 apiFetch throws (see
// frontend/app/financeiro/cartoes/actions.ts for the same pattern), so a
// transaction a bill still points at (transactions.Delete now translates
// Postgres' 23503 into domain.ErrConflict) shows a readable Portuguese
// sentence instead of leaving the row silently stuck with no explanation.
function isConflict(err: unknown): boolean {
  return err instanceof Error && /-> 409:/.test(err.message);
}

export async function deleteTransactionAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteTransaction(id);
  } catch (err) {
    if (isConflict(err)) {
      return {
        error: "Este lançamento veio da quitação de uma conta; desfaça o pagamento em Contas para removê-lo.",
      };
    }
    return { error: err instanceof Error ? err.message : "Falha ao excluir." };
  }
  revalidateFinance();
  return {};
}
