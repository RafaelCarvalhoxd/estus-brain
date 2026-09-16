"use server";

import { revalidatePath } from "next/cache";
import { createWorkout, deleteWorkout, updateWorkout, type WorkoutInput } from "@/lib/training";
import { apiErrorMessage } from "@/lib/week";

export async function saveWorkoutAction(input: WorkoutInput, id?: string): Promise<{ error?: string }> {
  if (!input.name.trim()) return { error: "Dê um nome ao treino." };
  if (input.days.length === 0) return { error: "Escolha pelo menos um dia da semana." };

  const exercises = input.exercises
    .filter((e) => e.name.trim())
    .map((e) => ({ ...e, name: e.name.trim(), reps: e.reps.trim(), weight: e.weight.trim(), notes: e.notes.trim() }));
  if (exercises.some((e) => !Number.isInteger(e.sets) || e.sets < 1 || e.sets > 50)) {
    return { error: "Cada exercício precisa de 1 a 50 séries." };
  }

  const clean: WorkoutInput = {
    name: input.name.trim(),
    focus: input.focus.trim(),
    days: input.days,
    notes: input.notes.trim(),
    exercises,
  };

  try {
    if (id) await updateWorkout(id, clean);
    else await createWorkout(clean);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível salvar o treino.") };
  }
  revalidatePath("/treino");
  revalidatePath("/");
  return {};
}

export async function deleteWorkoutAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteWorkout(id);
  } catch (err) {
    return { error: apiErrorMessage(err, "Não foi possível excluir o treino.") };
  }
  revalidatePath("/treino");
  revalidatePath("/");
  return {};
}
