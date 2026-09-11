import "server-only";

// Mirrors backend/internal/httpapi/dto_events.go by hand, same convention as
// lib/reminders.ts and lib/bills.ts — kept as its own file so this module
// never needs to touch the shared types.ts / api.ts other work is editing
// in parallel.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Event {
  id: string;
  title: string;
  location: string;
  notes: string;
  starts_at: string;
  ends_at: string;
  google_event_id?: string;
}

export interface CreateEventInput {
  title: string;
  location?: string;
  notes?: string;
  starts_at: string;
  ends_at: string;
}

export interface GoogleStatus {
  connected: boolean;
}

export interface GoogleSyncResult {
  imported: number;
}

async function agendaFetch<T>(path: string, init?: RequestInit): Promise<T> {
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

export function listEvents(from: Date, to: Date): Promise<Event[]> {
  const query = `?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`;
  return agendaFetch<Event[]>(`/api/events${query}`, { cache: "no-store" });
}

export function createEvent(input: CreateEventInput): Promise<Event> {
  return agendaFetch<Event>("/api/events", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateEvent(id: string, input: CreateEventInput): Promise<Event> {
  return agendaFetch<Event>(`/api/events/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteEvent(id: string): Promise<void> {
  return agendaFetch<void>(`/api/events/${id}`, { method: "DELETE" });
}

export function googleStatus(): Promise<GoogleStatus> {
  return agendaFetch<GoogleStatus>("/api/google/status", { cache: "no-store" });
}

export function triggerGoogleSync(): Promise<GoogleSyncResult> {
  return agendaFetch<GoogleSyncResult>("/api/google/sync", { method: "POST" });
}
