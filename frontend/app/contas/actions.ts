"use server";

import { revalidatePath } from "next/cache";
import { createBill, markBillPaid } from "@/lib/bills";
import type { BillDirection } from "@/lib/bills";

export type BillFormState = {
  status: "idle" | "error" | "success";
  message?: string;
};

function parseAmountToCents(raw: string): number | null {
  const normalized = raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", ".");
  const value = Number.parseFloat(normalized);
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}

export async function createBillAction(
  _prevState: BillFormState,
  formData: FormData,
): Promise<BillFormState> {
  const description = String(formData.get("description") ?? "").trim();
  const amountCents = parseAmountToCents(String(formData.get("amount") ?? ""));
  const dueDate = String(formData.get("due_date") ?? "");
  const direction = String(formData.get("direction") ?? "") as BillDirection;
  const categoryId = String(formData.get("category_id") ?? "") || undefined;
  const recurring = formData.get("recurring") === "1";

  if (!description) return { status: "error", message: "Descrição é obrigatória." };
  if (amountCents === null) return { status: "error", message: "Valor inválido." };
  if (!dueDate) return { status: "error", message: "Selecione o vencimento." };
  if (direction !== "pagar" && direction !== "receber") {
    return { status: "error", message: "Selecione a direção." };
  }

  try {
    await createBill({
      description,
      amount_cents: amountCents,
      due_date: dueDate,
      direction,
      category_id: categoryId,
      recurring,
    });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/contas");
  return { status: "success", message: "Conta salva." };
}

export async function markBillPaidAction(id: string): Promise<void> {
  await markBillPaid(id);
  revalidatePath("/contas");
}
