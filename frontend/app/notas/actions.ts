"use server";

import { revalidatePath } from "next/cache";
import { createNote, deleteNote, updateNote } from "@/lib/notes";

export type NoteFormState = {
  status: "idle" | "error" | "success";
  message?: string;
};

export async function createNoteAction(
  _prevState: NoteFormState,
  formData: FormData,
): Promise<NoteFormState> {
  const title = String(formData.get("title") ?? "").trim();
  const body = String(formData.get("body") ?? "").trim();
  const pinned = formData.get("pinned") === "1";

  if (!title && !body) {
    return { status: "error", message: "Escreva um título ou um texto." };
  }

  try {
    await createNote({ title, body, pinned });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao criar nota." };
  }

  revalidatePath("/notas");
  return { status: "success", message: "Nota criada." };
}

export async function updateNoteAction(
  _prevState: NoteFormState,
  formData: FormData,
): Promise<NoteFormState> {
  const id = String(formData.get("id") ?? "");
  const title = String(formData.get("title") ?? "").trim();
  const body = String(formData.get("body") ?? "").trim();
  const pinned = formData.get("pinned") === "1";

  if (!id) {
    return { status: "error", message: "Nota inválida." };
  }
  if (!title && !body) {
    return { status: "error", message: "Escreva um título ou um texto." };
  }

  try {
    await updateNote(id, { title, body, pinned });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar nota." };
  }

  revalidatePath("/notas");
  return { status: "success", message: "Nota salva." };
}

export async function deleteNoteAction(id: string): Promise<void> {
  await deleteNote(id);
  revalidatePath("/notas");
}
