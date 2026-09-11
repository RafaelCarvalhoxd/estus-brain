"use server";

import { revalidatePath } from "next/cache";
import { updateCategoryBudget } from "@/lib/api";

function parseAmountToCents(raw: string): number | null {
  const normalized = raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", ".");
  const value = Number.parseFloat(normalized);
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}

export async function setCategoryBudgetAction(
  categoryId: string,
  rawAmount: string,
): Promise<{ error?: string }> {
  const trimmed = rawAmount.trim();
  const cents = trimmed === "" ? null : parseAmountToCents(trimmed);
  if (trimmed !== "" && cents === null) {
    return { error: "Valor inválido." };
  }

  await updateCategoryBudget(categoryId, cents);
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/categorias");
  return {};
}
