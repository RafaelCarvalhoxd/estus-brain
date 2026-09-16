"use server";

import { revalidatePath } from "next/cache";
import {
  createDocumentFolder,
  deleteDocument,
  deleteDocumentFolder,
  moveDocument,
  renameDocumentFolder,
  uploadDocument,
} from "@/lib/documents";

export type DocumentFormState = {
  status: "idle" | "error" | "success";
  message?: string;
};

function folderIdOf(formData: FormData): string | null {
  const raw = String(formData.get("folder_id") ?? "");
  return raw === "" ? null : raw;
}

export async function uploadDocumentsAction(
  _prevState: DocumentFormState,
  formData: FormData,
): Promise<DocumentFormState> {
  const folderId = folderIdOf(formData);
  const files = formData.getAll("file").filter((f): f is File => f instanceof File && f.size > 0);

  if (files.length === 0) {
    return { status: "error", message: "Escolha pelo menos um arquivo." };
  }

  try {
    for (const file of files) {
      await uploadDocument(file, folderId);
    }
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao enviar." };
  }

  revalidatePath("/documentos");
  return {
    status: "success",
    message: files.length === 1 ? "Arquivo enviado." : `${files.length} arquivos enviados.`,
  };
}

export async function createFolderAction(
  _prevState: DocumentFormState,
  formData: FormData,
): Promise<DocumentFormState> {
  const name = String(formData.get("name") ?? "").trim();
  if (!name) return { status: "error", message: "Dê um nome à pasta." };

  try {
    await createDocumentFolder(folderIdOf(formData), name);
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao criar pasta." };
  }

  revalidatePath("/documentos");
  return { status: "success", message: "Pasta criada." };
}

export async function renameFolderAction(id: string, name: string): Promise<{ error?: string }> {
  if (!name.trim()) return { error: "Nome é obrigatório." };
  try {
    await renameDocumentFolder(id, name.trim());
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao renomear." };
  }
  revalidatePath("/documentos");
  return {};
}

export async function deleteFolderAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteDocumentFolder(id);
  } catch (err) {
    // The API refuses to delete a folder that still holds anything, which
    // is the common case worth explaining in plain words.
    const message = err instanceof Error && err.message.includes("not empty")
      ? "A pasta ainda tem coisas dentro. Esvazie antes de excluir."
      : err instanceof Error
        ? err.message
        : "Falha ao excluir pasta.";
    return { error: message };
  }
  revalidatePath("/documentos");
  return {};
}

export async function renameDocumentAction(id: string, folderId: string | null, name: string): Promise<{ error?: string }> {
  if (!name.trim()) return { error: "Nome é obrigatório." };
  try {
    await moveDocument(id, folderId, name.trim());
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao renomear." };
  }
  revalidatePath("/documentos");
  return {};
}

export async function deleteDocumentAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteDocument(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao excluir." };
  }
  revalidatePath("/documentos");
  return {};
}
