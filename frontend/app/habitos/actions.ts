"use server";

import { revalidatePath } from "next/cache";
import { createHabit, deleteHabit, setHabitLog, updateHabit, type HabitInput } from "@/lib/habits";
import { apiErrorMessage, dayKeyIn, TZ } from "@/lib/week";

function refresh() {
  revalidatePath("/habitos");
  revalidatePath("/");
}

export async function saveHabitAction(input: HabitInput, id?: string): Promise<{ error?: string }> {
  const name = input.name.trim();
  if (!name) return { error: "Dê um nome ao hábito." };
  if (input.days.length === 0) return { error: "Escolha pelo menos um dia da semana." };
  if (input.kind === "count" && (!Number.isInteger(input.target) || input.target < 1 || input.target > 1000)) {
    return { error: "A meta precisa ser um número de 1 a 1000." };
  }

  const clean: HabitInput = { ...input, name, unit: input.unit.trim(), target: input.kind === "check" ? 1 : input.target };
  try {
    if (id) await updateHabit(id, clean);
    else await createHabit({ ...clean, start_day: dayKeyIn(TZ) });
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível salvar o hábito.") };
  }
  refresh();
  return {};
}

export async function deleteHabitAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteHabit(id);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível excluir o hábito.") };
  }
  refresh();
  return {};
}

// Days in the future can't be marked: you haven't done them yet.
export async function setHabitLogAction(id: string, day: string, count: number): Promise<{ error?: string }> {
  if (day > dayKeyIn(TZ)) return { error: "Esse dia ainda não chegou." };
  try {
    await setHabitLog(id, day, Math.max(0, Math.round(count)));
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível registrar.") };
  }
  refresh();
  return {};
}
