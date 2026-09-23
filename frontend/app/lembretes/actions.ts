"use server";

import { revalidatePath } from "next/cache";
import { createReminder, deleteReminder, setReminderDone, updateReminder } from "@/lib/reminders";

export type Repeat = { days: number[]; monthDay?: number };

function repeats(r: Repeat): boolean {
  return r.days.length > 0 || r.monthDay !== undefined;
}

export type CreateReminderState = {
  status: "idle" | "error" | "success";
  message?: string;
};

function todayInputValue(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// A repeating reminder with no date starts today; the server moves it to
// the first chosen day.
function toDueAt(date: string, time: string, repeat: Repeat): { dueAt?: string; error?: string } {
  if (!date && repeats(repeat)) date = todayInputValue();
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
  const monthDay = Number(formData.get("repeat_month_day") ?? "");
  const repeat: Repeat = {
    days: formData.getAll("repeat_days").map(Number),
    monthDay: monthDay >= 1 && monthDay <= 31 ? monthDay : undefined,
  };

  if (!title) return { status: "error", message: "Título é obrigatório." };

  const { dueAt, error } = toDueAt(date, time, repeat);
  if (error) return { status: "error", message: error };

  try {
    await createReminder({ title, due_at: dueAt, repeat_days: repeat.days, repeat_month_day: repeat.monthDay });
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
  repeat: Repeat,
): Promise<{ error?: string }> {
  if (!title.trim()) return { error: "Título é obrigatório." };
  const { dueAt, error } = toDueAt(date, time, repeat);
  if (error) return { error };

  await updateReminder(id, {
    title: title.trim(),
    due_at: dueAt,
    repeat_days: repeat.days,
    repeat_month_day: repeat.monthDay,
  });
  revalidatePath("/lembretes");
  return {};
}
