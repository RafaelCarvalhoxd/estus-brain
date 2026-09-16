"use client";

import { isLookId, type LookId } from "./looks";

// The knobs behind the home brain, kept in one tiny store so the panel in
// the header and the canvas below it stay in sync without threading state
// through the server-rendered page. Choices live in localStorage, so the
// brain comes back the way its owner left it.

export interface BrainSettings {
  /** How many folds the brain grows. Rebuilds the geometry. */
  density: number;
  /** Multiplies every glow: halos, lit nodes, signals. */
  glow: number;
  /** Signal speed, wave frequency, heartbeat. */
  speed: number;
  /** Overall size. */
  scale: number;
  /** How far the folds stand apart in depth — how 3D it feels when turned. */
  depth: number;
  /** Arcs jumping between lobes. */
  sparks: boolean;
  /** The model (style) the brain is drawn in. */
  look: LookId;
}

export const DEFAULT_SETTINGS: BrainSettings = {
  density: 1,
  glow: 1,
  speed: 1,
  scale: 1.05,
  depth: 65,
  sparks: true,
  look: "cristal",
};

export const SETTING_RANGE = {
  density: { min: 0.5, max: 1.6, step: 0.05 },
  glow: { min: 0, max: 2, step: 0.05 },
  speed: { min: 0.2, max: 2.5, step: 0.05 },
  scale: { min: 0.8, max: 1.35, step: 0.01 },
  depth: { min: 0, max: 180, step: 5 },
} as const;

const KEY = "estus-brain-settings";

let current: BrainSettings = DEFAULT_SETTINGS;
const listeners = new Set<() => void>();

function clampNumbers(value: Partial<BrainSettings>): Partial<BrainSettings> {
  const out: Partial<BrainSettings> = {};
  for (const [k, range] of Object.entries(SETTING_RANGE) as [keyof typeof SETTING_RANGE, { min: number; max: number }][]) {
    const v = value[k];
    if (typeof v === "number" && Number.isFinite(v)) out[k] = Math.min(range.max, Math.max(range.min, v));
  }
  if (typeof value.sparks === "boolean") out.sparks = value.sparks;
  if (isLookId(value.look)) out.look = value.look;
  return out;
}

export function getSettings(): BrainSettings {
  return current;
}

// The server has no localStorage, so it always renders the defaults; the
// stored choice is folded in right after mount.
export function getServerSettings(): BrainSettings {
  return DEFAULT_SETTINGS;
}

export function subscribeSettings(onChange: () => void): () => void {
  listeners.add(onChange);
  return () => listeners.delete(onChange);
}

function commit(next: BrainSettings, persist: boolean) {
  current = next;
  if (persist) {
    try {
      localStorage.setItem(KEY, JSON.stringify(current));
    } catch {
      // storage blocked — the change still applies for this session
    }
  }
  for (const l of listeners) l();
}

export function setSettings(patch: Partial<BrainSettings>) {
  commit({ ...current, ...clampNumbers(patch) }, true);
}

export function resetSettings() {
  commit(DEFAULT_SETTINGS, true);
}

export function hydrateSettings() {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw) as Partial<BrainSettings>;
    commit({ ...DEFAULT_SETTINGS, ...clampNumbers(parsed) }, false);
  } catch {
    // unreadable or corrupt — stay on the defaults
  }
}
