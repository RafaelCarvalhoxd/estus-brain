"use client";

import type { ModuleId } from "@/lib/modules";
import type { BrainColors } from "./brain3d";

const MODULE_IDS: ModuleId[] = ["financeiro", "agenda", "treino", "dieta", "notas", "lembretes", "documentos", "senhas", "quadros", "habitos"];

// The brain's colors come from the same CSS custom properties as the rest
// of the app, so a theme switch reaches it too. Read on mount and again
// whenever the theme attribute flips.
export function readBrainColors(): BrainColors {
  const cs = getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string) => {
    const raw = cs.getPropertyValue(name).trim();
    return /^#[0-9a-f]{6}$/i.test(raw) ? raw : fallback;
  };
  const page = v("--page", "#121214");
  const n = parseInt(page.slice(1), 16);
  const lum = (0.2126 * ((n >> 16) & 255) + 0.7152 * ((n >> 8) & 255) + 0.0722 * (n & 255)) / 255;
  const modules = {} as Record<ModuleId, string>;
  for (const id of MODULE_IDS) modules[id] = v(`--m-${id}`, "#8a8a92");
  return {
    dark: lum < 0.4,
    page,
    glow: v("--brain-glow", "#4d8dff"),
    spark: v("--brain-spark", "#ffb454"),
    tissue: v("--tissue", "#2f5fd0"),
    modules,
  };
}
