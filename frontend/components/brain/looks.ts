// The styles the home brain can be drawn in. Every look is the same brain —
// same geometry, same interaction — with different light: colors, line
// weight, how much it glows, and a couple of effects that only some looks
// use (a hologram scan, energy flowing along the folds). The owner picks one
// in the settings panel (or side by side at /cerebro) and it's remembered with
// the other brain settings. Cristal is the default.

export type LookId = "eletrico" | "neon" | "holograma" | "plasma" | "dourado" | "cristal";

export interface BrainLook {
  id: LookId;
  name: string;
  description: string;
  /** Overrides for the theme's brain colors; anything left out comes from CSS. */
  colors: { glow?: string; glow2?: string; spark?: string; tissue?: string };
  /** 0 = one glow color; 1 = glow at the base shading fully to glow2 at the crown. */
  gradient: number;
  /** Multiplier on how many fold lines the surface carries. */
  lineFreq: number;
  /** Half-width of a fold line, in noise units. */
  lineWidth: number;
  /** How much light the lines and edges emit. */
  emit: number;
  /** Strength of the warm vein network. */
  warm: number;
  /** Strength of the glowing silhouette. */
  fresnel: number;
  /** Brightness of the tissue under the lines. */
  tissue: number;
  /** Hologram scanlines and a sweeping band of light. */
  scan: number;
  /** Pulses of energy travelling along the folds. */
  flow: number;
  /** Bloom strength on a dark page. */
  bloom: number;
  /** Brightness of the core flare. */
  flare: number;
  /** Brightness of the platform ring. */
  ring: number;
}

export const LOOKS: Record<LookId, BrainLook> = {
  eletrico: {
    id: "eletrico",
    name: "Azul elétrico",
    description: "Linhas azuis nítidas, veios dourados e núcleo branco.",
    colors: {},
    gradient: 0,
    lineFreq: 1,
    lineWidth: 0.012,
    emit: 0.46,
    warm: 1,
    fresnel: 1,
    tissue: 1,
    scan: 0,
    flow: 0,
    bloom: 0.8,
    flare: 1,
    ring: 1,
  },
  neon: {
    id: "neon",
    name: "Neon intenso",
    description: "Mais perto da foto de referência: brilho forte, azul puxando pro roxo, sinapses douradas.",
    colors: { glow: "#3f8cff", glow2: "#a56bff", spark: "#ffb13b", tissue: "#2a4fd6" },
    gradient: 0.45,
    lineFreq: 1.05,
    lineWidth: 0.015,
    emit: 0.5,
    warm: 1.6,
    fresnel: 1.1,
    tissue: 1,
    scan: 0,
    flow: 0.35,
    bloom: 0.95,
    flare: 1.15,
    ring: 1.25,
  },
  holograma: {
    id: "holograma",
    name: "Holograma",
    description: "Ciano translúcido com uma faixa de luz varrendo de baixo pra cima, como uma projeção.",
    colors: { glow: "#38e8ff", glow2: "#38e8ff", spark: "#aefcff", tissue: "#0d5566" },
    gradient: 0,
    lineFreq: 1.1,
    lineWidth: 0.009,
    emit: 0.5,
    warm: 0,
    fresnel: 2.1,
    tissue: 0.45,
    scan: 1,
    flow: 0,
    bloom: 0.9,
    flare: 0.55,
    ring: 1.1,
  },
  plasma: {
    id: "plasma",
    name: "Plasma",
    description: "Roxo na base, rosa no topo, com energia correndo pelas dobras.",
    colors: { glow: "#7a5cff", glow2: "#ff4fd8", spark: "#ffc2f4", tissue: "#35208a" },
    gradient: 1,
    lineFreq: 0.85,
    lineWidth: 0.014,
    emit: 0.52,
    warm: 0.5,
    fresnel: 1.2,
    tissue: 1,
    scan: 0,
    flow: 1,
    bloom: 1.05,
    flare: 1,
    ring: 0.9,
  },
  dourado: {
    id: "dourado",
    name: "Dourado",
    description: "Rede neural em âmbar e laranja sobre tecido escuro, com pulsos lentos.",
    colors: { glow: "#ffb44d", glow2: "#ff7a3d", spark: "#fff0c2", tissue: "#5a3514" },
    gradient: 0.5,
    lineFreq: 1,
    lineWidth: 0.012,
    emit: 0.48,
    warm: 0.35,
    fresnel: 1,
    tissue: 0.9,
    scan: 0,
    flow: 0.5,
    bloom: 0.95,
    flare: 1.1,
    ring: 0.8,
  },
  cristal: {
    id: "cristal",
    name: "Cristal mínimo",
    description: "Linhas brancas finas e quase nenhum brilho. O mais discreto, e o padrão.",
    colors: { glow: "#d6e4ff", glow2: "#d6e4ff", spark: "#ffffff", tissue: "#1c2436" },
    gradient: 0,
    lineFreq: 1.3,
    lineWidth: 0.007,
    emit: 0.34,
    warm: 0,
    fresnel: 0.7,
    tissue: 0.8,
    scan: 0,
    flow: 0,
    bloom: 0.45,
    flare: 0.4,
    ring: 0.45,
  },
};

// Cristal first: it's the default.
export const LOOK_LIST: BrainLook[] = [
  LOOKS.cristal,
  LOOKS.eletrico,
  LOOKS.neon,
  LOOKS.holograma,
  LOOKS.plasma,
  LOOKS.dourado,
];

export function isLookId(value: unknown): value is LookId {
  return typeof value === "string" && Object.prototype.hasOwnProperty.call(LOOKS, value);
}
