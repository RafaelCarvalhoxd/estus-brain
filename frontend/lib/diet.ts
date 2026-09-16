import "server-only";

// Mirrors backend/internal/httpapi/dto_training_diet.go by hand, same
// convention as the other modules.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface MealItem {
  id?: string;
  food: string;
  quantity: string;
  kcal: number;
  protein_g: number;
  carbs_g: number;
  fat_g: number;
}

export interface Meal {
  id: string;
  name: string;
  /** "HH:MM", local time */
  time: string;
  /** 0 = Sunday … 6 = Saturday */
  days: number[];
  notes: string;
  items: MealItem[];
  created_at: string;
  updated_at: string;
}

export type MealInput = Pick<Meal, "name" | "time" | "days" | "notes" | "items">;

export interface DietTargets {
  kcal: number;
  protein_g: number;
  carbs_g: number;
  fat_g: number;
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

export function listMeals(): Promise<Meal[]> {
  return apiFetch<Meal[]>("/api/meals", { cache: "no-store" });
}

export function createMeal(input: MealInput): Promise<Meal> {
  return apiFetch<Meal>("/api/meals", { method: "POST", body: JSON.stringify(input) });
}

export function updateMeal(id: string, input: MealInput): Promise<Meal> {
  return apiFetch<Meal>(`/api/meals/${id}`, { method: "PUT", body: JSON.stringify(input) });
}

export function deleteMeal(id: string): Promise<void> {
  return apiFetch<void>(`/api/meals/${id}`, { method: "DELETE" });
}

export function getDietTargets(): Promise<DietTargets> {
  return apiFetch<DietTargets>("/api/diet/targets", { cache: "no-store" });
}

export function setDietTargets(targets: DietTargets): Promise<DietTargets> {
  return apiFetch<DietTargets>("/api/diet/targets", { method: "PUT", body: JSON.stringify(targets) });
}
