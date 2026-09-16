"use server";

import { revalidatePath } from "next/cache";
import { updateCategoryBudget, createCategory, updateCategory, deleteCategory, type CategoryInput } from "@/lib/api";

function parseAmountToCents(raw: string): number | null {
  const normalized = raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", ".");
  const value = Number.parseFloat(normalized);
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}

// readBudget turns what was typed into the cents the API wants: an empty
// field clears the budget, anything unparseable is refused rather than
// silently clearing it.
function readBudget(raw: string): { cents: number | null } | { error: string } {
  const trimmed = raw.trim();
  if (trimmed === "") return { cents: null };
  const cents = parseAmountToCents(trimmed);
  if (cents === null) return { error: "Valor inválido." };
  return { cents };
}

export async function setCategoryBudgetAction(
  categoryId: string,
  rawAmount: string,
): Promise<{ error?: string }> {
  const budget = readBudget(rawAmount);
  if ("error" in budget) return budget;
  const cents = budget.cents;

  await updateCategoryBudget(categoryId, cents);
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/categorias");
  return {};
}

function revalidateCategories() {
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/lancamentos");
  revalidatePath("/financeiro/categorias");
  revalidatePath("/financeiro/contas");
}

// The budget lives behind its own endpoint, so saving a category with one is
// two calls. The category is written first: if the budget call then fails,
// the category still exists and the message says only the budget was lost,
// which beats refusing the whole save.
export async function createCategoryAction(
  input: CategoryInput,
  rawBudget = "",
): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Nome é obrigatório." };
  const budget = readBudget(rawBudget);
  if ("error" in budget) return budget;
  try {
    const created = await createCategory(input);
    if (budget.cents !== null) {
      await updateCategoryBudget(created.id, budget.cents);
    }
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateCategories();
  return {};
}

export async function updateCategoryAction(
  id: string,
  input: CategoryInput,
  rawBudget = "",
): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Nome é obrigatório." };
  const budget = readBudget(rawBudget);
  if ("error" in budget) return budget;
  try {
    await updateCategory(id, input);
    await updateCategoryBudget(id, budget.cents);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateCategories();
  return {};
}

export async function deleteCategoryAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteCategory(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao excluir." };
  }
  revalidateCategories();
  return {};
}
