import "server-only";

// Mirrors backend/internal/httpapi/handlers_boards.go by hand, same convention
// as the other modules. Scenes are saved by the editor through the route
// handler under app/api/boards — they're too big for a server action.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface BoardSummary {
  id: string;
  name: string;
  preview: string;
  created_at: string;
  updated_at: string;
}

export interface Board extends BoardSummary {
  scene: Record<string, unknown>;
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

export function listBoards(): Promise<BoardSummary[]> {
  return apiFetch<BoardSummary[]>("/api/boards", { cache: "no-store" });
}

// Resolves to null for a board that doesn't exist, so the page can 404.
export async function getBoard(id: string): Promise<Board | null> {
  const res = await fetch(`${API_URL}/api/boards/${encodeURIComponent(id)}`, { cache: "no-store" });
  if (res.status === 404 || res.status === 422) return null;
  if (!res.ok) throw new Error(`estus-vault api /api/boards/${id} -> ${res.status}: ${await res.text()}`);
  return res.json() as Promise<Board>;
}

export function createBoard(name: string): Promise<BoardSummary> {
  return apiFetch<BoardSummary>("/api/boards", { method: "POST", body: JSON.stringify({ name }) });
}

export function renameBoard(id: string, name: string): Promise<void> {
  return apiFetch<void>(`/api/boards/${id}`, { method: "PATCH", body: JSON.stringify({ name }) });
}

export function deleteBoard(id: string): Promise<void> {
  return apiFetch<void>(`/api/boards/${id}`, { method: "DELETE" });
}
