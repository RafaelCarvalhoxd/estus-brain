"use server";

import { revalidatePath } from "next/cache";
import { createNote, deleteNote, updateNote, createNoteCategory, deleteNoteCategory } from "@/lib/notes";

export type NoteFormState = {
  status: "idle" | "error" | "success";
  message?: string;
};

function readCategoryId(formData: FormData): string | null {
  const raw = String(formData.get("category_id") ?? "");
  return raw === "" ? null : raw;
}

export async function createNoteAction(
  _prevState: NoteFormState,
  formData: FormData,
): Promise<NoteFormState> {
  const title = String(formData.get("title") ?? "").trim();
  const body = String(formData.get("body") ?? "").trim();
  const pinned = formData.get("pinned") === "1";
  const categoryId = readCategoryId(formData);

  if (!title && !body) {
    return { status: "error", message: "Escreva um título ou um texto." };
  }

  try {
    await createNote({ title, body, pinned, category_id: categoryId });
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
  const categoryId = readCategoryId(formData);

  if (!id) {
    return { status: "error", message: "Nota inválida." };
  }
  if (!title && !body) {
    return { status: "error", message: "Escreva um título ou um texto." };
  }

  try {
    await updateNote(id, { title, body, pinned, category_id: categoryId });
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

export async function createNoteCategoryAction(name: string, color: string): Promise<{ error?: string; id?: string }> {
  if (!name.trim()) return { error: "Nome é obrigatório." };
  try {
    const category = await createNoteCategory({ name: name.trim(), color });
    revalidatePath("/notas");
    return { id: category.id };
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao criar categoria." };
  }
}

export async function deleteNoteCategoryAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteNoteCategory(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao excluir categoria." };
  }
  revalidatePath("/notas");
  return {};
}
