import "server-only";

// Mirrors backend/internal/httpapi/dto_vault.go by hand, same convention as
// lib/api.ts and lib/reminders.ts. Reveal isn't here — it runs client-side
// against app/api/vault/[id]/reveal/route.ts, not through this server-only
// module.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface VaultEntry {
  id: string;
  title: string;
  username: string;
  url: string;
  notes?: string;
  updated_at: string;
}

export interface VaultEntryInput {
  title: string;
  username: string;
  password: string;
  url: string;
  notes: string;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api ${path} -> ${res.status}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function listVaultEntries(): Promise<VaultEntry[]> {
  return apiFetch<VaultEntry[]>("/api/vault", { cache: "no-store" });
}

export function createVaultEntry(input: VaultEntryInput): Promise<VaultEntry> {
  return apiFetch<VaultEntry>("/api/vault", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateVaultEntry(id: string, input: VaultEntryInput): Promise<VaultEntry> {
  return apiFetch<VaultEntry>(`/api/vault/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteVaultEntry(id: string): Promise<void> {
  return apiFetch<void>(`/api/vault/${id}`, { method: "DELETE" });
}
