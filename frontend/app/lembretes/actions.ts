"use server";

import { revalidatePath } from "next/cache";
import { createReminder, deleteReminder, setReminderDone, updateReminder } from "@/lib/reminders";

export type CreateReminderState = {
  status: "idle" | "error" | "success";
  message?: string;
};

export const initialCreateReminderState: CreateReminderState = { status: "idle" };

function toDueAt(date: string, time: string): { dueAt?: string; error?: string } {
  if (!date) return {};
  const isoTime = time || "00:00";
  const local = new Date(`${date}T${isoTime}:00`);
  if (Number.isNaN(local.getTime())) return { error: "Data ou horário inválido." };
  return { dueAt: local.toISOString() };
}

export async function createReminderAction(
  _prevState: CreateReminderState,
  formData: FormData,
): Promise<CreateReminderState> {
  const title = String(formData.get("title") ?? "").trim();
  const date = String(formData.get("due_date") ?? "");
  const time = String(formData.get("due_time") ?? "");

  if (!title) return { status: "error", message: "Título é obrigatório." };

  const { dueAt, error } = toDueAt(date, time);
  if (error) return { status: "error", message: error };

  try {
    await createReminder({ title, due_at: dueAt });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/lembretes");
  return { status: "success", message: "Lembrete criado." };
}

export async function toggleReminderAction(id: string, done: boolean): Promise<void> {
  await setReminderDone(id, done);
  revalidatePath("/lembretes");
}

export async function deleteReminderAction(id: string): Promise<void> {
  await deleteReminder(id);
  revalidatePath("/lembretes");
}

export async function updateReminderAction(
  id: string,
  title: string,
  date: string,
  time: string,
): Promise<{ error?: string }> {
  if (!title.trim()) return { error: "Título é obrigatório." };
  const { dueAt, error } = toDueAt(date, time);
  if (error) return { error };

  await updateReminder(id, { title: title.trim(), due_at: dueAt });
  revalidatePath("/lembretes");
  return {};
}
