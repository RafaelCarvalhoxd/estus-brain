"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { createBoard, deleteBoard, renameBoard } from "@/lib/boards";

export type BoardFormState = {
  status: "idle" | "error";
  message?: string;
};

// A new board opens straight in the editor — there's nothing else to do
// with an empty one.
export async function createBoardAction(_prevState: BoardFormState, formData: FormData): Promise<BoardFormState> {
  const name = String(formData.get("name") ?? "").trim() || "Quadro sem título";

  let id: string;
  try {
    id = (await createBoard(name)).id;
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao criar quadro." };
  }

  revalidatePath("/quadros");
  redirect(`/quadros/${id}`);
}

export async function renameBoardAction(id: string, name: string): Promise<{ error?: string }> {
  if (!name.trim()) return { error: "Nome é obrigatório." };
  try {
    await renameBoard(id, name.trim());
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao renomear." };
  }
  revalidatePath("/quadros");
  revalidatePath(`/quadros/${id}`);
  return {};
}

export async function deleteBoardAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteBoard(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao excluir." };
  }
  revalidatePath("/quadros");
  return {};
}
