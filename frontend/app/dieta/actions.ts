"use server";

import { revalidatePath } from "next/cache";
import { createMeal, deleteMeal, setDietTargets, updateMeal, type DietTargets, type MealInput } from "@/lib/diet";
import { apiErrorMessage } from "@/lib/week";

const validNumber = (n: number) => Number.isFinite(n) && n >= 0;

export async function saveMealAction(input: MealInput, id?: string): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Dê um nome à refeição." };
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(input.time)) return { error: "Informe o horário da refeição." };
  if (input.days.length === 0) return { error: "Escolha pelo menos um dia da semana." };

  const items = input.items
    .filter((it) => it.food.trim())
    .map((it) => ({ ...it, food: it.food.trim(), quantity: it.quantity.trim() }));
  if (items.some((it) => ![it.kcal, it.protein_g, it.carbs_g, it.fat_g].every(validNumber))) {
    return { error: "Os macros precisam ser números iguais ou maiores que zero." };
  }

  const clean: MealInput = { name: input.name.trim(), time: input.time, days: input.days, notes: input.notes.trim(), items };

  try {
    if (id) await updateMeal(id, clean);
    else await createMeal(clean);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível salvar a refeição.") };
  }
  revalidatePath("/dieta");
  revalidatePath("/");
  return {};
}

export async function deleteMealAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteMeal(id);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível excluir a refeição.") };
  }
  revalidatePath("/dieta");
  revalidatePath("/");
  return {};
}

export async function saveTargetsAction(targets: DietTargets): Promise<{ error?: string }> {
  if (![targets.kcal, targets.protein_g, targets.carbs_g, targets.fat_g].every(validNumber)) {
    return { error: "As metas precisam ser números iguais ou maiores que zero." };
  }
  try {
    await setDietTargets(targets);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível salvar as metas.") };
  }
  revalidatePath("/dieta");
  return {};
}
