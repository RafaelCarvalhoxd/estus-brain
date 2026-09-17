"use server";

import { revalidatePath } from "next/cache";
import { createBill, markBillPaid, updateBill, deleteBill, endSeries, resumeSeries } from "@/lib/bills";
import type { BillDirection, BillPaymentMethod } from "@/lib/bills";

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

// A conta que se repete só vira gasto sozinho quando o dono a quita — e isso
// exige categoria e forma de pagamento (domain.Bill.Validate recusa com 422
// sem os dois). Checar aqui evita o round-trip e mostra a frase certa em vez
// do erro cru da API.
const SERIES_NEEDS_CATEGORY_AND_METHOD =
  "Conta que se repete precisa de categoria e forma de pagamento — é assim que ela vira gasto quando você quita.";

function seriesRequirementError(
  recurring: boolean,
  direction: BillDirection,
  categoryId: string | undefined,
  paymentMethod: string | undefined,
): string | null {
  if (recurring && direction === "pagar" && (!categoryId || !paymentMethod)) {
    return SERIES_NEEDS_CATEGORY_AND_METHOD;
  }
  return null;
}

// isValidationError/isConflict recognize the 422/409 apiFetch throws, so the
// UI shows a readable Portuguese sentence instead of the raw
// "estus-vault api ... -> 422: ..." fetch error — same pattern as
// frontend/app/financeiro/cartoes/actions.ts. Every validation rule the
// backend enforces is also checked above before the request goes out, so
// these are a last-resort net, not the primary defense.
function isValidationError(err: unknown): boolean {
  return err instanceof Error && /-> 422:/.test(err.message);
}

function isConflict(err: unknown): boolean {
  return err instanceof Error && /-> 409:/.test(err.message);
}

function friendlyMessage(err: unknown, fallback: string): string {
  if (isValidationError(err)) return "Não foi possível salvar: confira os campos da conta.";
  if (isConflict(err)) return "Não foi possível salvar: conflito com uma conta existente.";
  return err instanceof Error ? err.message : fallback;
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
  const amountVaries = formData.get("amount_varies") === "1";
  const paymentMethod = (String(formData.get("payment_method") ?? "") || undefined) as
    | BillPaymentMethod
    | undefined;

  if (!description) return { status: "error", message: "Descrição é obrigatória." };
  if (amountCents === null) return { status: "error", message: "Valor inválido." };
  if (!dueDate) return { status: "error", message: "Selecione o vencimento." };
  if (direction !== "pagar" && direction !== "receber") {
    return { status: "error", message: "Selecione a direção." };
  }
  const seriesError = seriesRequirementError(recurring, direction, categoryId, paymentMethod);
  if (seriesError) return { status: "error", message: seriesError };

  try {
    await createBill({
      description,
      amount_cents: amountCents,
      due_date: dueDate,
      direction,
      category_id: categoryId,
      recurring,
      amount_varies: amountVaries,
      // The first occurrence of a series that varies starts life as a guess
      // too — there is no previous occurrence to carry amount_estimated from,
      // so it is seeded from the same switch as amount_varies here, once.
      // Every later occurrence instead gets it from BillService.Materialize
      // (seeded from amount_varies, not from whatever this occurrence's own
      // amount_estimated has become after being paid or corrected).
      amount_estimated: amountVaries,
      payment_method: paymentMethod,
    });
  } catch (err) {
    return { status: "error", message: friendlyMessage(err, "Falha ao salvar.") };
  }

  revalidatePath("/financeiro/contas");
  return { status: "success", message: "Conta salva." };
}

export async function markBillPaidAction(id: string): Promise<void> {
  await markBillPaid(id);
  revalidatePath("/financeiro/contas");
}

export type UpdateBillFields = {
  description: string;
  amount: string;
  due_date: string;
  direction: BillDirection;
  category_id?: string;
  // recurring reflects the bill's own series membership — it is read-only in
  // the edit row (series membership only ever changes at creation, see
  // BillService.Update) and exists here only so this check can be applied
  // the same way on both create and update.
  recurring: boolean;
  payment_method?: BillPaymentMethod;
  // amount_varies is likewise read-only here — the edit row has no switch
  // for it — and must always be resent as the bill's own current value.
  // BillService.Update takes it verbatim from the request (unlike
  // SeriesID/PaidAt/TransactionID/SeriesEnded), so omitting it would reach
  // the backend as false and silently clear a series that does vary.
  amount_varies: boolean;
};

export async function updateBillAction(id: string, fields: UpdateBillFields): Promise<{ error?: string }> {
  const amountCents = parseAmountToCents(fields.amount);
  if (!fields.description.trim()) return { error: "Descrição é obrigatória." };
  if (amountCents === null) return { error: "Valor inválido." };
  if (!fields.due_date) return { error: "Selecione o vencimento." };
  const seriesError = seriesRequirementError(fields.recurring, fields.direction, fields.category_id, fields.payment_method);
  if (seriesError) return { error: seriesError };

  try {
    await updateBill(id, {
      description: fields.description.trim(),
      amount_cents: amountCents,
      due_date: fields.due_date,
      direction: fields.direction,
      category_id: fields.category_id,
      recurring: fields.recurring,
      payment_method: fields.payment_method,
      amount_varies: fields.amount_varies,
    });
  } catch (err) {
    return { error: friendlyMessage(err, "Falha ao salvar.") };
  }
  revalidatePath("/financeiro/contas");
  return {};
}

export async function deleteBillAction(id: string): Promise<void> {
  await deleteBill(id);
  revalidatePath("/financeiro/contas");
}

// endSeriesAction / resumeSeriesAction: dedicated, explicit way to cancel or
// restart a recurring bill, replacing the old (unimplementable once
// Materialize runs on every month view) idea that deleting the last
// occurrence "ends" a series.
export async function endSeriesAction(id: string): Promise<void> {
  await endSeries(id);
  revalidatePath("/financeiro/contas");
}

export async function resumeSeriesAction(id: string): Promise<void> {
  await resumeSeries(id);
  revalidatePath("/financeiro/contas");
}
