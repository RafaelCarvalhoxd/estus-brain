"use server";

import { revalidatePath } from "next/cache";
import { updateCategoryBudget, createCategory, updateCategory, deleteCategory, type CategoryInput } from "@/lib/api";

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

function revalidateCategories() {
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/lancamentos");
  revalidatePath("/financeiro/categorias");
  revalidatePath("/financeiro/contas");
}

export async function createCategoryAction(input: CategoryInput): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Nome é obrigatório." };
  try {
    await createCategory(input);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateCategories();
  return {};
}

export async function updateCategoryAction(id: string, input: CategoryInput): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Nome é obrigatório." };
  try {
    await updateCategory(id, input);
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
