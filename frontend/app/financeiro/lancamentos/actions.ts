"use server";

import { revalidatePath } from "next/cache";
import { createTransaction, updateTransaction, deleteTransaction } from "@/lib/api";
import type { PaymentMethod } from "@/lib/types";

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

export async function createTransactionAction(
  _prevState: CreateTransactionState,
  formData: FormData,
): Promise<CreateTransactionState> {
  const description = String(formData.get("description") ?? "").trim();
  const amountCents = parseAmountToCents(String(formData.get("amount") ?? ""));
  const categoryId = String(formData.get("category_id") ?? "");
  const paymentMethod = String(formData.get("payment_method") ?? "") as PaymentMethod;
  const purchaseDate = String(formData.get("purchase_date") ?? "");
  const creditCardId = String(formData.get("credit_card_id") ?? "") || undefined;
  const installments = Number(formData.get("installments") ?? 1);
  const isRecurring = formData.get("is_recurring") === "1";

  if (!description) return { status: "error", message: "Descrição é obrigatória." };
  if (amountCents === null) return { status: "error", message: "Valor inválido." };
  if (!categoryId) return { status: "error", message: "Selecione uma categoria." };
  if (!purchaseDate) return { status: "error", message: "Selecione a data da compra." };
  if (paymentMethod === "credito" && !creditCardId) {
    return { status: "error", message: "Selecione o cartão." };
  }

  try {
    await createTransaction({
      description,
      amount_cents: amountCents,
      category_id: categoryId,
      payment_method: paymentMethod,
      purchase_date: purchaseDate,
      credit_card_id: creditCardId,
      installments: paymentMethod === "credito" ? installments : 1,
      is_recurring: isRecurring,
    });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/");
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/lancamentos");
  revalidatePath("/financeiro/categorias");
  return { status: "success", message: "Lançamento salvo." };
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
