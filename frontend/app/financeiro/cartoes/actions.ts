"use server";

import { revalidatePath } from "next/cache";
import {
  createCreditCard,
  updateCreditCard,
  deleteCreditCard,
  type CreditCardInput,
} from "@/lib/api";

// invalid is a plain sync helper, not exported: a "use server" file may only
// export async functions, so validation stays private and the actions below
// call into it instead of being validators themselves.
function invalid(input: CreditCardInput): string | null {
  if (!input.name.trim()) return "Nome é obrigatório.";
  for (const [label, day] of [
    ["fechamento", input.closing_day],
    ["vencimento", input.due_day],
  ] as const) {
    if (!Number.isInteger(day) || day < 1 || day > 28) {
      return `O dia de ${label} precisa estar entre 1 e 28.`;
    }
  }
  return null;
}

function revalidateCards() {
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/cartoes");
  revalidatePath("/financeiro/lancamentos");
}

// isConflict recognizes the 409 apiFetch throws, so the UI can show a readable
// Portuguese sentence instead of the raw "estus-vault api ... -> 409: ..."
// fetch error. Each call site knows which conflict its own route can raise:
// saving conflicts on a duplicate name, deleting on attached transactions.
function isConflict(err: unknown): boolean {
  return err instanceof Error && /-> 409:/.test(err.message);
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

export async function createCreditCardAction(input: CreditCardInput): Promise<{ error?: string }> {
  const problem = invalid(input);
  if (problem) return { error: problem };
  try {
    await createCreditCard(input);
  } catch (err) {
    if (isConflict(err)) return { error: "Já existe um cartão com esse nome." };
    return { error: errorMessage(err, "Falha ao salvar.") };
  }
  revalidateCards();
  return {};
}

export async function updateCreditCardAction(
  id: string,
  input: CreditCardInput,
): Promise<{ error?: string }> {
  const problem = invalid(input);
  if (problem) return { error: problem };
  try {
    await updateCreditCard(id, input);
  } catch (err) {
    if (isConflict(err)) return { error: "Já existe um cartão com esse nome." };
    return { error: errorMessage(err, "Falha ao salvar.") };
  }
  revalidateCards();
  return {};
}

export async function deleteCreditCardAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteCreditCard(id);
  } catch (err) {
    if (isConflict(err)) {
      return { error: "Não é possível excluir: este cartão tem lançamentos registrados nele." };
    }
    return { error: errorMessage(err, "Falha ao excluir.") };
  }
  revalidateCards();
  return {};
}
