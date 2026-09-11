import "server-only";

// Mirrors the JSON DTOs in backend/internal/httpapi/dto_notes.go by hand,
// same convention as lib/api.ts for the dashboard endpoints.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Note {
  id: string;
  title: string;
  body: string;
  pinned: boolean;
  category_id?: string;
  created_at: string;
  updated_at: string;
}

export interface NoteCategory {
  id: string;
  name: string;
  color: string;
  created_at: string;
}

export interface CreateNoteInput {
  title: string;
  body: string;
  pinned?: boolean;
  category_id?: string | null;
}

export interface UpdateNoteInput {
  title: string;
  body: string;
  pinned?: boolean;
  category_id?: string | null;
}

export interface NoteCategoryInput {
  name: string;
  color: string;
}

async function notesFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api ${path} -> ${res.status}: ${body}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export function listNotes(): Promise<Note[]> {
  return notesFetch<Note[]>("/api/notes", { cache: "no-store" });
}

export function getNote(id: string): Promise<Note> {
  return notesFetch<Note>(`/api/notes/${id}`, { cache: "no-store" });
}

export function createNote(input: CreateNoteInput): Promise<Note> {
  return notesFetch<Note>("/api/notes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateNote(id: string, input: UpdateNoteInput): Promise<Note> {
  return notesFetch<Note>(`/api/notes/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteNote(id: string): Promise<void> {
  return notesFetch<void>(`/api/notes/${id}`, { method: "DELETE" });
}

export function listNoteCategories(): Promise<NoteCategory[]> {
  return notesFetch<NoteCategory[]>("/api/note-categories", { cache: "no-store" });
}

export function createNoteCategory(input: NoteCategoryInput): Promise<NoteCategory> {
  return notesFetch<NoteCategory>("/api/note-categories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function deleteNoteCategory(id: string): Promise<unknown> {
  return notesFetch(`/api/note-categories/${id}`, { method: "DELETE" });
}
