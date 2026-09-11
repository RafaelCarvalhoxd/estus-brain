import "server-only";

// Mirrors backend/internal/httpapi/dto_reminders.go by hand, same convention
// as lib/api.ts and lib/types.ts.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Reminder {
  id: string;
  title: string;
  due_at?: string;
  done: boolean;
  created_at: string;
}

export interface CreateReminderInput {
  title: string;
  due_at?: string;
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

export function listReminders(): Promise<Reminder[]> {
  return apiFetch<Reminder[]>("/api/reminders", { cache: "no-store" });
}

export function createReminder(input: CreateReminderInput): Promise<Reminder> {
  return apiFetch<Reminder>("/api/reminders", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function setReminderDone(id: string, done: boolean): Promise<Reminder> {
  return apiFetch<Reminder>(`/api/reminders/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ done }),
  });
}

export function deleteReminder(id: string): Promise<void> {
  return apiFetch<void>(`/api/reminders/${id}`, { method: "DELETE" });
}

export function updateReminder(id: string, input: CreateReminderInput): Promise<Reminder> {
  return apiFetch<Reminder>(`/api/reminders/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}
