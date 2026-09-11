"use server";

import { revalidatePath } from "next/cache";
import { createEvent, deleteEvent, triggerGoogleSync, updateEvent } from "@/lib/agenda";

export type CreateEventState = {
  status: "idle" | "error" | "success";
  message?: string;
};

function toRange(date: string, startTime: string, endTime: string): { startsAt?: Date; endsAt?: Date; error?: string } {
  if (!date) return { error: "Selecione a data." };
  if (!startTime || !endTime) return { error: "Informe o horário de início e fim." };

  const startsAt = new Date(`${date}T${startTime}:00`);
  const endsAt = new Date(`${date}T${endTime}:00`);
  if (Number.isNaN(startsAt.getTime()) || Number.isNaN(endsAt.getTime())) {
    return { error: "Data ou horário inválido." };
  }
  if (endsAt <= startsAt) return { error: "O horário de fim deve ser depois do início." };
  return { startsAt, endsAt };
}

export async function createEventAction(
  _prevState: CreateEventState,
  formData: FormData,
): Promise<CreateEventState> {
  const title = String(formData.get("title") ?? "").trim();
  const location = String(formData.get("location") ?? "").trim();
  const notes = String(formData.get("notes") ?? "").trim();
  const date = String(formData.get("date") ?? "");
  const startTime = String(formData.get("start_time") ?? "");
  const endTime = String(formData.get("end_time") ?? "");

  if (!title) return { status: "error", message: "Título é obrigatório." };
  const { startsAt, endsAt, error } = toRange(date, startTime, endTime);
  if (error) return { status: "error", message: error };

  try {
    await createEvent({
      title,
      location: location || undefined,
      notes: notes || undefined,
      starts_at: startsAt!.toISOString(),
      ends_at: endsAt!.toISOString(),
    });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/agenda");
  return { status: "success", message: "Evento criado." };
}

export async function deleteEventAction(id: string): Promise<void> {
  await deleteEvent(id);
  revalidatePath("/agenda");
}

export async function updateEventAction(
  id: string,
  title: string,
  location: string,
  date: string,
  startTime: string,
  endTime: string,
): Promise<{ error?: string }> {
  if (!title.trim()) return { error: "Título é obrigatório." };
  const { startsAt, endsAt, error } = toRange(date, startTime, endTime);
  if (error) return { error };

  await updateEvent(id, {
    title: title.trim(),
    location: location.trim() || undefined,
    starts_at: startsAt!.toISOString(),
    ends_at: endsAt!.toISOString(),
  });
  revalidatePath("/agenda");
  return {};
}

export type SyncState = {
  status: "idle" | "error" | "success";
  message?: string;
};

export async function syncGoogleAction(): Promise<SyncState> {
  try {
    const result = await triggerGoogleSync();
    revalidatePath("/agenda");
    return { status: "success", message: `${result.imported} evento(s) sincronizado(s).` };
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao sincronizar." };
  }
}
