"use server";

import { revalidatePath } from "next/cache";
import { createVaultEntry, deleteVaultEntry, updateVaultEntry } from "@/lib/vault";

export type VaultFormState = {
  status: "idle" | "error" | "success";
  message?: string;
};

export async function createVaultEntryAction(
  _prevState: VaultFormState,
  formData: FormData,
): Promise<VaultFormState> {
  const title = String(formData.get("title") ?? "").trim();
  const username = String(formData.get("username") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  const url = String(formData.get("url") ?? "").trim();
  const notes = String(formData.get("notes") ?? "").trim();

  if (!title) return { status: "error", message: "Título é obrigatório." };
  if (!password) return { status: "error", message: "Senha é obrigatória." };

  try {
    await createVaultEntry({ title, username, password, url, notes });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/senhas");
  return { status: "success", message: "Senha salva." };
}

export async function updateVaultEntryAction(
  _prevState: VaultFormState,
  formData: FormData,
): Promise<VaultFormState> {
  const id = String(formData.get("id") ?? "");
  const title = String(formData.get("title") ?? "").trim();
  const username = String(formData.get("username") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  const url = String(formData.get("url") ?? "").trim();
  const notes = String(formData.get("notes") ?? "").trim();

  if (!id) return { status: "error", message: "Registro inválido." };
  if (!title) return { status: "error", message: "Título é obrigatório." };

  try {
    await updateVaultEntry(id, { title, username, password, url, notes });
  } catch (err) {
    return { status: "error", message: err instanceof Error ? err.message : "Falha ao salvar." };
  }

  revalidatePath("/senhas");
  return { status: "success", message: "Alterações salvas." };
}

export async function deleteVaultEntryAction(id: string): Promise<void> {
  await deleteVaultEntry(id);
  revalidatePath("/senhas");
}
