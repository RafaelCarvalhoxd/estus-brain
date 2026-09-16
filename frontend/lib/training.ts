import "server-only";

// Mirrors backend/internal/httpapi/dto_training_diet.go by hand, same
// convention as the other modules.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface Exercise {
  id?: string;
  name: string;
  sets: number;
  reps: string;
  weight: string;
  rest_seconds: number;
  notes: string;
}

export interface Workout {
  id: string;
  name: string;
  focus: string;
  /** 0 = Sunday … 6 = Saturday */
  days: number[];
  notes: string;
  exercises: Exercise[];
  created_at: string;
  updated_at: string;
}

export type WorkoutInput = Pick<Workout, "name" | "focus" | "days" | "notes" | "exercises">;

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

export function listWorkouts(): Promise<Workout[]> {
  return apiFetch<Workout[]>("/api/workouts", { cache: "no-store" });
}

export function createWorkout(input: WorkoutInput): Promise<Workout> {
  return apiFetch<Workout>("/api/workouts", { method: "POST", body: JSON.stringify(input) });
}

export function updateWorkout(id: string, input: WorkoutInput): Promise<Workout> {
  return apiFetch<Workout>(`/api/workouts/${id}`, { method: "PUT", body: JSON.stringify(input) });
}

export function deleteWorkout(id: string): Promise<void> {
  return apiFetch<void>(`/api/workouts/${id}`, { method: "DELETE" });
}
