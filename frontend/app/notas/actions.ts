"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import {
  createNote,
  createNoteCategory,
  deleteNote,
  deleteNoteCategory,
  listNoteCategories,
  listNotes,
  updateNoteCategory,
} from "@/lib/notes";
import { capitalize, TZ } from "@/lib/week";

const CATEGORY_COLORS = ["#8b5cf6", "#3b7fe0", "#10a37f", "#e0972f", "#d94f70", "#0f9aa8", "#7cc242"];
const DAILY_CATEGORY = "Diário";

// A new notebook takes the first color no other notebook has yet.
function nextColor(used: string[]): string {
  const taken = new Set(used.map((c) => c.toLowerCase()));
  return CATEGORY_COLORS.find((c) => !taken.has(c)) ?? CATEGORY_COLORS[used.length % CATEGORY_COLORS.length];
}

function message(err: unknown, fallback: string): string {
  if (!(err instanceof Error)) return fallback;
  if (err.message.includes("already exists")) return "Já existe um caderno com esse nome.";
  return fallback;
}

function notesHref(noteId: string, categoryId: string | null): string {
  const params = new URLSearchParams({ n: noteId });
  if (categoryId) params.set("c", categoryId);
  return `/notas?${params}`;
}

// A new note is created empty and opened; the editor removes it again if it's
// left without anything written in it.
export async function createNoteAction(categoryId: string | null): Promise<{ error?: string }> {
  const realCategory = categoryId && categoryId !== "fixadas" && categoryId !== "geral" ? categoryId : null;
  let id: string;
  try {
    id = (await createNote({ title: "", body: "", category_id: realCategory, pinned: categoryId === "fixadas" })).id;
  } catch {
    return { error: "Não foi possível criar a nota." };
  }
  revalidatePath("/notas");
  redirect(notesHref(id, categoryId));
}

// "Nota do dia": one note per day, titled with the date, in a "Diário"
// notebook made on first use. Opening it again the same day finds the same note.
export async function openDailyNoteAction(): Promise<{ error?: string }> {
  const title = capitalize(
    new Date().toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long", year: "numeric", timeZone: TZ }),
  );

  let id: string;
  let categoryId: string;
  try {
    const categories = await listNoteCategories();
    let daily = categories.find((c) => c.name === DAILY_CATEGORY);
    if (!daily) {
      daily = await createNoteCategory({ name: DAILY_CATEGORY, color: nextColor(categories.map((c) => c.color)) });
    }
    categoryId = daily.id;
    const notes = await listNotes();
    const existing = notes.find((n) => n.category_id === daily.id && n.title === title);
    id = existing ? existing.id : (await createNote({ title, body: "", category_id: daily.id })).id;
  } catch {
    return { error: "Não foi possível abrir a nota do dia." };
  }
  revalidatePath("/notas");
  redirect(notesHref(id, categoryId));
}

export async function deleteNoteAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteNote(id);
  } catch {
    return { error: "Não foi possível excluir a nota." };
  }
  revalidatePath("/notas");
  return {};
}

export async function createNoteCategoryAction(name: string): Promise<{ error?: string; id?: string }> {
  if (!name.trim()) return { error: "Dê um nome ao caderno." };
  try {
    const existing = await listNoteCategories();
    const category = await createNoteCategory({
      name: name.trim(),
      color: nextColor(existing.map((c) => c.color)),
    });
    revalidatePath("/notas");
    return { id: category.id };
  } catch (err) {
    return { error: message(err, "Não foi possível criar o caderno.") };
  }
}

export async function renameNoteCategoryAction(id: string, name: string, color: string): Promise<{ error?: string }> {
  if (!name.trim()) return { error: "Dê um nome ao caderno." };
  try {
    await updateNoteCategory(id, { name: name.trim(), color });
  } catch (err) {
    return { error: message(err, "Não foi possível renomear o caderno.") };
  }
  revalidatePath("/notas");
  return {};
}

// The notes in a deleted notebook aren't lost: they go back to "Geral".
export async function deleteNoteCategoryAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteNoteCategory(id);
  } catch {
    return { error: "Não foi possível excluir o caderno." };
  }
  revalidatePath("/notas");
  return {};
}
