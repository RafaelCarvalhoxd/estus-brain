"use server";

import { revalidatePath } from "next/cache";
import { createEvent, deleteEvent, triggerGoogleSync } from "@/lib/agenda";

export type CreateEventState = {
  status: "idle" | "error" | "success";
  message?: string;
};

export const initialCreateEventState: CreateEventState = { status: "idle" };

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
  if (!date) return { status: "error", message: "Selecione a data." };
  if (!startTime || !endTime) return { status: "error", message: "Informe o horário de início e fim." };

  const startsAt = new Date(`${date}T${startTime}:00`);
  const endsAt = new Date(`${date}T${endTime}:00`);
  if (Number.isNaN(startsAt.getTime()) || Number.isNaN(endsAt.getTime())) {
    return { status: "error", message: "Data ou horário inválido." };
  }
  if (endsAt <= startsAt) {
    return { status: "error", message: "O horário de fim deve ser depois do início." };
  }

  try {
    await createEvent({
      title,
      location: location || undefined,
      notes: notes || undefined,
      starts_at: startsAt.toISOString(),
      ends_at: endsAt.toISOString(),
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
