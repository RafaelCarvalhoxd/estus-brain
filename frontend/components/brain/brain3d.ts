import * as THREE from "three";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import { ShaderPass } from "three/addons/postprocessing/ShaderPass.js";
import { mergeVertices } from "three/addons/utils/BufferGeometryUtils.js";
import type { ModuleId } from "@/lib/modules";
import { createNoise3D } from "./noise";
import { LOOKS, type BrainLook } from "./looks";

// The home brain in WebGL: a whole brain, both hemispheres, seen from the
// front and a little above, ringed by a glowing platform.
//
// The shape is an icosphere pushed into brain proportions (flat medial wall,
// longitudinal fissure, temporal lobe dropping down the side, blunt frontal
// pole, pointier occipital), then carved with simplex noise so gyri rise and
// sulci sink. The noise value at each vertex travels to the shader, which
// lights the crest of every gyrus — that tracery of light is the look.
//
// Brain-local space: x lateral (+right), y up, z front. Each module lives in
// a region of it, loosely anatomical: frontal → Financeiro, motor strip →
// Treino, insula (taste) → Dieta, temporal → Notas, occipital → Lembretes,
// frontal → Financeiro (left) and Hábitos (right), parietal → Agenda (right)
// and Quadros (left), cerebellum → Senhas,
// brainstem/core → Documentos.

export interface BrainColors {
  dark: boolean;
  page: string;
  glow: string;
  spark: string;
  tissue: string;
  modules: Record<ModuleId, string>;
}

export interface BrainSettings {
  glow: number;
  speed: number;
  scale: number;
  sparks: boolean;
}

export interface BrainFrame {
  t: number;
  dt: number;
  yaw: number;
  pitch: number;
  zoom: number;
  hover: ModuleId | null;
  leave: { id: ModuleId; progress: number } | null;
  boot: number;
  settings: BrainSettings;
  reduce: boolean;
}

export interface FocusBox {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Brain3D {
  resize(width: number, height: number, dpr: number, focus: FocusBox): void;
  setColors(colors: BrainColors): void;
  /** Switch the style the brain is drawn in, without rebuilding it. */
  setLook(look: BrainLook): void;
  rebuild(density: number, depth: number): void;
  render(frame: BrainFrame): void;
  /** Where a module's anchor lands on the canvas, and how much it faces the viewer (0..1). */
  project(id: ModuleId): { x: number; y: number; facing: number } | null;
  /** Which module is under a canvas point, if any. */
  hit(x: number, y: number): ModuleId | null;
  /** A ring of light sweeping out from a module's region. */
  wave(id: ModuleId | null): void;
  dispose(): void;
}

const REGION_INDEX: Record<ModuleId, number> = {
  financeiro: 1,
  notas: 2,
  lembretes: 3,
  agenda: 4,
  senhas: 5,
  documentos: 6,
  treino: 7,
  dieta: 8,
  quadros: 9,
  habitos: 10,
};
const MODULE_BY_INDEX: Record<number, ModuleId> = {
  1: "financeiro",
  2: "notas",
  3: "lembretes",
  4: "agenda",
  5: "senhas",
  6: "documentos",
  7: "treino",
  8: "dieta",
  9: "quadros",
  10: "habitos",
};

// Must match regionOf() in the tissue shader.
function regionOf(p: THREE.Vector3): number {
  const lateral = Math.abs(p.x) > 0.34;
  if (p.z < -0.46) return 3; // occipital
  if (lateral && p.y < 0.1 && p.y > -0.26 && p.z > -0.3 && p.z < 0.3) return 8; // insula
  if (lateral && p.y < -0.04) return 2; // temporal
  if (p.z > 0.3) return p.x > 0 ? 10 : 1; // frontal, right and left
  if (p.z > 0.02 && p.y > -0.1) return 7; // motor strip
  if (p.x < 0 && p.y > 0.2) return 9; // left superior parietal
  return 4; // parietal
}

// Rough spots each module's connector reaches for; snapped to the surface
// once the geometry exists. Modules on the left of the screen reach the
// left hemisphere, those on the right the right one — on the silhouette
// where they can, so the lines meet the brain's glowing edge.
const ANCHOR_TARGETS: Record<ModuleId, [number, number, number]> = {
  financeiro: [-0.42, 0.42, 0.62],
  treino: [-0.5, 0.52, 0.16],
  notas: [-0.82, -0.3, 0.18],
  documentos: [-0.1, -0.66, 0.02],
  agenda: [0.4, 0.62, -0.18],
  dieta: [0.7, -0.06, 0.0],
  quadros: [-0.42, 0.6, -0.3],
  habitos: [0.42, 0.42, 0.62],
  lembretes: [0.68, 0.32, -0.52],
  senhas: [0.52, -0.46, -0.6],
};

const CORE = new THREE.Vector3(0, -0.05, 0.02);
const PLATFORM_Y = -0.62;
const HALF_HEIGHT = 1.02; // crown to platform, halved, with some air
const CENTER_Y = -0.2;
const FOV = 30;

const smooth = (a: number, b: number, x: number) => {
  const t = Math.min(1, Math.max(0, (x - a) / (b - a)));
  return t * t * (3 - 2 * t);
};

// ---------------------------------------------------------------- shaders

const SIMPLEX_GLSL = /* glsl */ `
vec3 mod289(vec3 x){return x - floor(x*(1.0/289.0))*289.0;}
vec4 mod289(vec4 x){return x - floor(x*(1.0/289.0))*289.0;}
vec4 permute(vec4 x){return mod289(((x*34.0)+1.0)*x);}
vec4 taylorInvSqrt(vec4 r){return 1.79284291400159 - 0.85373472095314*r;}
float snoise(vec3 v){
  const vec2 C = vec2(1.0/6.0, 1.0/3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);
  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);
  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);
  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;
  i = mod289(i);
  vec4 p = permute(permute(permute(
    i.z + vec4(0.0, i1.z, i2.z, 1.0))
    + i.y + vec4(0.0, i1.y, i2.y, 1.0))
    + i.x + vec4(0.0, i1.x, i2.x, 1.0));
  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;
  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);
  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);
  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);
  vec4 s0 = floor(b0)*2.0 + 1.0;
  vec4 s1 = floor(b1)*2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));
  vec4 a0 = b0.xzyw + s0.xzyw*sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw*sh.zzww;
  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);
  vec4 norm = taylorInvSqrt(vec4(dot(p0,p0), dot(p1,p1), dot(p2,p2), dot(p3,p3)));
  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
  vec4 m = max(0.6 - vec4(dot(x0,x0), dot(x1,x1), dot(x2,x2), dot(x3,x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m*m, vec4(dot(p0,x0), dot(p1,x1), dot(p2,x2), dot(p3,x3)));
}
`;

const TISSUE_VS = /* glsl */ `
attribute float aFold;
varying float vFold;
varying vec3 vLocal;
varying vec3 vNormalW;
varying vec3 vViewDir;
void main() {
  vFold = aFold;
  vLocal = position;
  vec4 world = modelMatrix * vec4(position, 1.0);
  vNormalW = normalize(mat3(modelMatrix) * normal);
  vViewDir = normalize(cameraPosition - world.xyz);
  gl_Position = projectionMatrix * viewMatrix * world;
}
`;

const TISSUE_FS = /* glsl */ `
uniform float uTime;
uniform vec3 uGlowColor;
uniform vec3 uTissueColor;
uniform vec3 uSparkColor;
uniform vec3 uPage;
uniform vec3 uHoverColor;
uniform float uHoverRegion;
uniform float uHoverAmt;
uniform float uDimAmt;
uniform float uFixedRegion;
uniform float uFade;
uniform float uBoot;
uniform float uGlow;
uniform float uBeat;
uniform vec3 uWaveOrigin;
uniform float uWaveTime;
uniform vec3 uCore;
uniform float uDark;
uniform float uLineFreq;
uniform float uLineWidth;
uniform float uEmit;
uniform float uWarmAmt;
uniform float uFresnelAmt;
uniform float uTissueAmt;
uniform float uScan;
uniform float uFlow;
uniform vec3 uGlow2;
uniform float uGradient;
varying float vFold;
varying vec3 vLocal;
varying vec3 vNormalW;
varying vec3 vViewDir;
${SIMPLEX_GLSL}

float regionOf(vec3 p) {
  if (uFixedRegion > 0.5) return uFixedRegion;
  bool lateral = abs(p.x) > 0.34;
  if (p.z < -0.46) return 3.0;
  if (lateral && p.y < 0.1 && p.y > -0.26 && p.z > -0.3 && p.z < 0.3) return 8.0;
  if (lateral && p.y < -0.04) return 2.0;
  if (p.z > 0.3) return p.x > 0.0 ? 10.0 : 1.0;
  if (p.z > 0.02 && p.y > -0.1) return 7.0;
  if (p.x < 0.0 && p.y > 0.2) return 9.0;
  return 4.0;
}

void main() {
  // Power on from the brainstem outward.
  float fromStem = length(vLocal - vec3(0.0, -1.0, -0.2));
  float edge = uBoot * 2.9;
  if (fromStem > edge) discard;
  float bootFlash = smoothstep(0.22, 0.0, abs(fromStem - edge)) * (1.0 - step(0.999, uBoot));

  float region = regionOf(vLocal);
  float hot = step(0.5, uHoverRegion) * (1.0 - step(0.5, abs(region - uHoverRegion))) * uHoverAmt;
  float dim = mix(1.0, 0.5, uDimAmt * (1.0 - hot));
  vec3 baseLight = mix(uGlowColor, uGlow2, uGradient * smoothstep(-0.7, 0.75, vLocal.y));
  vec3 light = mix(baseLight, uHoverColor, hot);

  vec3 n = normalize(vNormalW);
  vec3 v = normalize(vViewDir);
  float facing = max(dot(n, v), 0.0);
  float fresnel = pow(1.0 - facing, 2.0);
  float diffuse = 0.35 + 0.65 * max(dot(n, normalize(vec3(0.0, 0.8, 0.6))), 0.0);

  // Gyri outlines, drawn per pixel so they stay crisp however close you get:
  // zero-crossings of warped noise meander like the edges of folds. The
  // geometry's own fold value shades the sulci darker underneath.
  float nl;
  if (uFixedRegion > 4.5 && uFixedRegion < 5.5) {
    nl = 0.5 * sin(vLocal.y * 95.0 + 2.0 * snoise(vLocal * 4.0));
  } else if (uFixedRegion > 5.5) {
    nl = 0.5 * sin(atan(vLocal.z + 0.22, vLocal.x) * 6.0 + vLocal.y * 1.5);
  } else {
    vec3 w = vec3(snoise(vLocal * 1.7), snoise(vLocal * 1.7 + 7.1), snoise(vLocal * 1.7 + 3.3));
    vec3 q = vLocal * uLineFreq + w * 0.9;
    nl = 0.8 * snoise(q) + 0.2 * snoise(q * 2.3 + 11.0);
  }
  float aa = max(fwidth(nl), 0.002);
  float line = 1.0 - smoothstep(uLineWidth, uLineWidth + aa * 1.6, abs(nl));
  float soft = 1.0 - smoothstep(0.0, 0.22, abs(nl));
  float sulcus = smoothstep(0.1, 0.7, vFold);
  float shimmer = 0.78 + 0.22 * snoise(vLocal * 3.0 + vec3(0.0, uTime * 0.35, 0.0));

  float wave = 0.0;
  if (uWaveTime >= 0.0 && uWaveTime < 1.8) {
    float r = uWaveTime * 1.5;
    wave = exp(-pow((length(vLocal - uWaveOrigin) - r) * 8.0, 2.0)) * (1.0 - uWaveTime / 1.8);
  }
  float core = exp(-dot(vLocal - uCore, vLocal - uCore) * 2.2);

  // Warm energy: a second, sparser network of veins glowing gold, strongest
  // low in the middle where the core is, and in drifting patches elsewhere.
  float vn = snoise(vLocal * 3.0 + vec3(0.0, 0.0, uTime * 0.04));
  float vein = 1.0 - smoothstep(0.02, 0.02 + max(fwidth(vn), 0.002) * 1.8, abs(vn));
  float warmNear = smoothstep(1.05, 0.15, length(vLocal - vec3(0.0, -0.3, 0.25)));
  float warmPatch = smoothstep(0.25, 0.75, snoise(vLocal * 1.4 + vec3(uTime * 0.06, 0.0, 0.0)));
  float warm = vein * max(warmNear, warmPatch * 0.8) * (1.0 - 0.7 * hot);

  vec3 tissue = uTissueColor * diffuse * (0.34 + 0.18 * soft) * (1.0 - 0.45 * sulcus) * uTissueAmt;
  tissue = mix(tissue, uHoverColor * diffuse * 0.45, hot * 0.5);

  // Light is added in screen space, where it stacks up fast; this keeps the
  // brain a network of light over dark tissue instead of a white blob.
  float EMIT = uEmit;
  vec3 col = tissue * dim;
  col += light * line * shimmer * (1.6 + 1.2 * wave + 1.1 * core + 0.35 * uBeat) * uGlow * dim * EMIT;
  col += light * soft * 0.12 * dim * EMIT;
  col += light * fresnel * (0.8 + 0.5 * hot) * uFresnelAmt * uGlow * dim * EMIT;
  col += uSparkColor * warm * (2.1 + 0.8 * uBeat) * uWarmAmt * uGlow * dim * EMIT;
  col += light * wave * 0.35 * EMIT;

  // Energy flowing along the folds: bright beads that travel the lines.
  if (uFlow > 0.0) {
    float phase = snoise(vLocal * 1.3) * 14.0 - uTime * 3.2;
    float bead = pow(max(0.0, sin(phase)), 18.0);
    col += mix(light, uSparkColor, 0.35) * line * bead * uFlow * 3.2 * uGlow * dim * EMIT;
  }
  // Hologram: fine scanlines across the surface and a band of light
  // sweeping up it.
  if (uScan > 0.0) {
    float lines = 0.55 + 0.45 * sin(vLocal.y * 160.0 - uTime * 6.0);
    col *= mix(1.0, lines, uScan * 0.6);
    float sweepY = mod(uTime * 0.45, 2.6) - 1.3;
    float band = exp(-pow((vLocal.y - sweepY) * 9.0, 2.0));
    col += light * band * uScan * 0.9 * uGlow * dim * EMIT;
  }

  col += uSparkColor * bootFlash * 1.4;

  // On a light page the brain is drawn as ink, not light.
  if (uDark < 0.5) col = mix(uPage, col * 0.9, 0.85);

  gl_FragColor = vec4(mix(uPage, col, uFade), 1.0);
}
`;

const SPARK_VS = /* glsl */ `
attribute float aSeed;
attribute float aWarm;
uniform float uTime;
uniform float uSize;
uniform float uBoot;
varying float vBright;
varying float vWarm;
void main() {
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  float rate = 0.6 + fract(aSeed * 13.7) * 1.8;
  float tw = pow(max(0.0, sin(uTime * rate + aSeed * 40.0)), 12.0);
  float lit = step(length(position - vec3(0.0, -1.0, -0.2)), uBoot * 2.9);
  vBright = (aWarm > 0.5 ? (0.05 + tw * 1.8) : (0.25 + tw * 0.6)) * lit;
  vWarm = aWarm;
  gl_PointSize = uSize * (0.55 + 1.3 * tw) / -mv.z;
  gl_Position = projectionMatrix * mv;
}
`;

const SPARK_FS = /* glsl */ `
uniform vec3 uGlowColor;
uniform vec3 uSparkColor;
uniform float uSparks;
uniform float uFade;
varying float vBright;
varying float vWarm;
void main() {
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0.0, d);
  a *= a;
  vec3 col = mix(uGlowColor, uSparkColor, vWarm);
  gl_FragColor = vec4(col * a * vBright * uSparks * uFade, 1.0);
}
`;

// Bloom blurs bright things across the whole frame, which lifts the
// background a little everywhere — on a small canvas, enough to show as a
// grey rectangle. This pulls anything barely above the page color back down
// to it, so the canvas meets the page exactly and only real glow remains.
const FLOOR_FS = /* glsl */ `
uniform sampler2D tDiffuse;
uniform vec3 uPage;
uniform float uCut;
varying vec2 vUv;
void main() {
  vec4 c = texture2D(tDiffuse, vUv);
  vec3 lift = max(c.rgb - uPage, 0.0);
  vec3 kept = max(lift - uCut, 0.0) / (1.0 - uCut);
  gl_FragColor = vec4(uPage + kept, 1.0);
}
`;

const FLOOR_VS = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

const PLATFORM_VS = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

const PLATFORM_FS = /* glsl */ `
uniform vec3 uGlowColor;
uniform float uFade;
uniform float uAppear;
uniform float uBeat;
uniform float uTime;
uniform float uRing;
varying vec2 vUv;
void main() {
  vec2 p = vUv * 2.0 - 1.0;
  float r = length(p);
  float ring = exp(-pow((r - 0.86) * 60.0, 2.0)) * (0.85 + 0.35 * uBeat);
  float inner = exp(-pow((r - 0.6) * 80.0, 2.0)) * 0.35;
  float halo = exp(-pow((r - 0.86) * 12.0, 2.0)) * 0.08;
  float fill = smoothstep(0.86, 0.0, r) * 0.025;
  // a slow sweep of light around the rim
  float ang = atan(p.y, p.x);
  float sweep = pow(max(0.0, cos(ang - uTime * 0.6)), 24.0) * exp(-pow((r - 0.86) * 30.0, 2.0)) * 0.8;
  vec3 col = uGlowColor * (ring + inner + halo + fill + sweep) * 0.42 * uRing;
  gl_FragColor = vec4(col * uFade * uAppear, 1.0);
}
`;

// ---------------------------------------------------------------- geometry

type Noise = (x: number, y: number, z: number) => number;

/** Pushes a unit direction into the shape of one hemisphere. */
function shapeHemisphere(side: 1 | -1, d: THREE.Vector3, out: THREE.Vector3) {
  const rx = 0.56;
  const ry = 0.6;
  const rz = 0.93;
  const lateral = d.x * side; // -1 medial … +1 lateral
  let x = d.x * rx * (lateral < 0 ? 0.26 : 1);
  let y = d.y * ry;
  let z = d.z * rz;
  x += side * (rx * 0.26 + 0.022);

  // flat-ish underside
  if (y < -0.16) y = -0.16 + (y + 0.16) * 0.6;

  const lat = Math.max(0, lateral);
  const low = Math.max(0, -d.y);
  const front = smooth(-0.7, 0.35, d.z);
  // temporal lobe drops down and out along the side
  x += side * 0.08 * lat * low * front;
  y -= 0.16 * low * lat * front * smooth(0.95, 0.2, d.z);

  // blunt frontal pole tipped down; longer, pointier occipital
  if (d.z > 0) y -= 0.07 * d.z * d.z;
  if (d.z < 0) {
    z *= 1 + 0.07 * -d.z;
    y += 0.05 * d.z;
  }
  // the crown rises toward the back, the way a real brain's does
  y += 0.05 * Math.max(0, d.y) * smooth(0.6, -0.4, d.z);

  out.set(x, y, z);
}

function buildHemisphere(side: 1 | -1, density: number, amp: number, noise: Noise, warp: Noise) {
  let geo: THREE.BufferGeometry = new THREE.IcosahedronGeometry(1, 72);
  geo.deleteAttribute("normal");
  geo.deleteAttribute("uv");
  geo = mergeVertices(geo);

  const pos = geo.getAttribute("position") as THREE.BufferAttribute;
  const fold = new Float32Array(pos.count);
  const d = new THREE.Vector3();
  const p = new THREE.Vector3();
  const outward = new THREE.Vector3();
  const freq = 4.8 * density;
  const hemiCenterX = side * (0.56 * 0.26 + 0.022);

  for (let i = 0; i < pos.count; i++) {
    d.fromBufferAttribute(pos, i).normalize();
    shapeHemisphere(side, d, p);

    // Warp the domain so the folds meander instead of looking like noise.
    const wx = p.x + 0.2 * warp(p.x * 1.4, p.y * 1.4, p.z * 1.4);
    const wy = p.y + 0.2 * warp(p.y * 1.4 + 7.1, p.z * 1.4, p.x * 1.4);
    const wz = p.z + 0.2 * warp(p.z * 1.4 + 3.3, p.x * 1.4, p.y * 1.4);
    // The two hemispheres fold differently, as real ones do.
    const o = side === 1 ? 11.3 : 0;
    const n = 0.72 * noise(wx * freq + o, wy * freq, wz * freq) + 0.28 * noise(wx * freq * 2.2 + o, wy * freq * 2.2 + 4.7, wz * freq * 2.2);
    const a = Math.abs(n);

    // The medial wall faces the other hemisphere: folded, but never bulging into it.
    const medial = smooth(-0.1, 0.3, d.x * side);
    fold[i] = a;
    const ridge = 1 - a;
    const raw = amp * (ridge * ridge - 0.42);
    const disp = medial > 0.5 ? raw : Math.min(raw, 0) * 0.6 + raw * 0.3 * medial;

    outward.set(p.x - hemiCenterX, p.y + 0.1, p.z).normalize();
    pos.setXYZ(i, p.x + outward.x * disp, p.y + outward.y * disp, p.z + outward.z * disp);
  }

  geo.setAttribute("aFold", new THREE.BufferAttribute(fold, 1));
  geo.computeVertexNormals();
  return geo;
}

function buildCerebellum(amp: number, noise: Noise) {
  let geo: THREE.BufferGeometry = new THREE.IcosahedronGeometry(1, 44);
  geo.deleteAttribute("normal");
  geo.deleteAttribute("uv");
  geo = mergeVertices(geo);
  const pos = geo.getAttribute("position") as THREE.BufferAttribute;
  const fold = new Float32Array(pos.count);
  const d = new THREE.Vector3();
  for (let i = 0; i < pos.count; i++) {
    d.fromBufferAttribute(pos, i).normalize();
    let x = d.x * 0.58;
    let y = d.y * 0.25;
    const z = d.z * 0.3;
    // two lobes with a notch between them
    y -= 0.05 * Math.exp(-x * x * 40) * Math.max(0, d.y);
    x *= 1 + 0.08 * Math.abs(d.x);
    // folia: tight horizontal leaves
    const leaf = Math.abs(Math.sin(y * 95 + 2.2 * noise(x * 3, y * 3, z * 3)));
    fold[i] = leaf * 0.55;
    const disp = amp * 0.5 * ((1 - leaf) * (1 - leaf) - 0.4);
    const len = Math.hypot(x, y, z) || 1;
    pos.setXYZ(i, x + (x / len) * disp, -0.44 + y + (y / len) * disp, -0.64 + z + (z / len) * disp);
  }
  geo.setAttribute("aFold", new THREE.BufferAttribute(fold, 1));
  geo.computeVertexNormals();
  return geo;
}

function buildStem() {
  const geo = new THREE.CylinderGeometry(0.1, 0.062, 0.82, 48, 36, true);
  geo.rotateX(-0.12);
  geo.translate(0, -0.72, -0.22);
  const pos = geo.getAttribute("position") as THREE.BufferAttribute;
  const fold = new Float32Array(pos.count);
  for (let i = 0; i < pos.count; i++) {
    const y = pos.getY(i);
    const x = pos.getX(i);
    // a few long fibres running down the stem
    fold[i] = 0.35 * Math.abs(Math.sin(Math.atan2(pos.getZ(i) + 0.22, x) * 6 + y * 1.5));
  }
  geo.setAttribute("aFold", new THREE.BufferAttribute(fold, 1));
  return geo;
}

// ---------------------------------------------------------------- engine

export function createBrain3D(canvas: HTMLCanvasElement, initialLook: BrainLook = LOOKS.cristal): Brain3D | null {
  let renderer: THREE.WebGLRenderer;
  try {
    renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: "high-performance" });
  } catch {
    return null;
  }

  // Colors are authored and blended as they look on screen and written out
  // untouched. Under three's linear workflow the canvas background came out
  // visibly lighter than the page color it was given, which showed as a grey
  // wash over the whole screen; here the clear color is the page, exactly.
  THREE.ColorManagement.enabled = false;
  renderer.outputColorSpace = THREE.LinearSRGBColorSpace;
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(FOV, 1, 0.1, 60);
  const brain = new THREE.Group();
  scene.add(brain);

  const shared = {
    uTime: { value: 0 },
    uGlowColor: { value: new THREE.Color("#4d8dff") },
    uTissueColor: { value: new THREE.Color("#2f5fd0") },
    uSparkColor: { value: new THREE.Color("#ffb454") },
    uPage: { value: new THREE.Color("#121214") },
    uHoverColor: { value: new THREE.Color("#4d8dff") },
    uHoverRegion: { value: 0 },
    uHoverAmt: { value: 0 },
    uDimAmt: { value: 0 },
    uFade: { value: 1 },
    uBoot: { value: 0 },
    uGlow: { value: 1 },
    uBeat: { value: 0 },
    uWaveOrigin: { value: new THREE.Vector3() },
    uWaveTime: { value: -1 },
    uCore: { value: CORE.clone() },
    uDark: { value: 1 },
    uLineFreq: { value: 7.2 },
    uLineWidth: { value: 0.012 },
    uEmit: { value: 0.46 },
    uWarmAmt: { value: 1 },
    uFresnelAmt: { value: 1 },
    uTissueAmt: { value: 1 },
    uScan: { value: 0 },
    uFlow: { value: 0 },
    uGlow2: { value: new THREE.Color("#4d8dff") },
    uGradient: { value: 0 },
  };
  let look = initialLook;
  let density = 1;
  let themeColors: BrainColors | null = null;

  const tissueMaterial = (fixedRegion: number) =>
    new THREE.ShaderMaterial({
      uniforms: { ...shared, uFixedRegion: { value: fixedRegion } },
      vertexShader: TISSUE_VS,
      fragmentShader: TISSUE_FS,
    });

  const materials = {
    hemi: tissueMaterial(0),
    cerebellum: tissueMaterial(REGION_INDEX.senhas),
    stem: tissueMaterial(REGION_INDEX.documentos),
  };
  materials.stem.side = THREE.DoubleSide;

  const sparkMaterial = new THREE.ShaderMaterial({
    uniforms: {
      uTime: shared.uTime,
      uGlowColor: shared.uGlowColor,
      uSparkColor: shared.uSparkColor,
      uFade: shared.uFade,
      uBoot: shared.uBoot,
      uSize: { value: 60 },
      uSparks: { value: 1 },
    },
    vertexShader: SPARK_VS,
    fragmentShader: SPARK_FS,
    blending: THREE.AdditiveBlending,
    transparent: true,
    depthWrite: false,
  });

  const platformMaterial = new THREE.ShaderMaterial({
    uniforms: {
      uGlowColor: shared.uGlowColor,
      uFade: shared.uFade,
      uBeat: shared.uBeat,
      uTime: shared.uTime,
      uAppear: { value: 0 },
      uRing: { value: 1 },
    },
    vertexShader: PLATFORM_VS,
    fragmentShader: PLATFORM_FS,
    blending: THREE.AdditiveBlending,
    transparent: true,
    depthWrite: false,
  });
  const platform = new THREE.Mesh(new THREE.PlaneGeometry(3.6, 3.6), platformMaterial);
  platform.rotation.x = -Math.PI / 2;
  platform.position.y = PLATFORM_Y;
  scene.add(platform);

  // The bright core where the brainstem enters: a white flare with a warm
  // one inside it, drawn over the tissue so it shines through.
  const flareTexture = (() => {
    const c = document.createElement("canvas");
    c.width = c.height = 128;
    const g = c.getContext("2d")!;
    const grad = g.createRadialGradient(64, 64, 0, 64, 64, 64);
    grad.addColorStop(0, "rgba(255,255,255,1)");
    grad.addColorStop(0.12, "rgba(225,236,255,0.85)");
    grad.addColorStop(0.35, "rgba(150,185,255,0.25)");
    grad.addColorStop(1, "rgba(90,130,255,0)");
    g.fillStyle = grad;
    g.fillRect(0, 0, 128, 128);
    return new THREE.CanvasTexture(c);
  })();
  const flareMaterial = new THREE.SpriteMaterial({
    map: flareTexture,
    blending: THREE.AdditiveBlending,
    depthTest: false,
    depthWrite: false,
    transparent: true,
  });
  const warmFlareMaterial = flareMaterial.clone();
  const flare = new THREE.Sprite(flareMaterial);
  const warmFlare = new THREE.Sprite(warmFlareMaterial);
  flare.position.set(0, -0.34, 0.3);
  warmFlare.position.copy(flare.position);
  flare.renderOrder = 10;
  warmFlare.renderOrder = 11;
  brain.add(flare, warmFlare);

  const noise = createNoise3D(20260914);
  const warp = createNoise3D(7);

  let hemiL: THREE.Mesh | null = null;
  let hemiR: THREE.Mesh | null = null;
  let cerebellum: THREE.Mesh | null = null;
  let stem: THREE.Mesh | null = null;
  let sparks: THREE.Points | null = null;
  const anchors = {} as Record<ModuleId, { p: THREE.Vector3; n: THREE.Vector3 }>;

  const disposeBrain = () => {
    for (const m of [hemiL, hemiR, cerebellum, stem]) {
      if (!m) continue;
      brain.remove(m);
      m.geometry.dispose();
    }
    if (sparks) {
      brain.remove(sparks);
      sparks.geometry.dispose();
    }
  };

  const rebuild = (nextDensity: number, depth: number) => {
    density = nextDensity;
    disposeBrain();
    const amp = 0.05 * (depth / 65);
    shared.uLineFreq.value = 7.2 * density * look.lineFreq;
    hemiL = new THREE.Mesh(buildHemisphere(-1, density, amp, noise, warp), materials.hemi);
    hemiR = new THREE.Mesh(buildHemisphere(1, density, amp, noise, warp), materials.hemi);
    cerebellum = new THREE.Mesh(buildCerebellum(amp, noise), materials.cerebellum);
    stem = new THREE.Mesh(buildStem(), materials.stem);
    brain.add(hemiL, hemiR, cerebellum, stem);

    // Synapses: points scattered along the gyri crests, a third of them
    // warm and twinkling, the rest a steady blue.
    const pts: number[] = [];
    const seeds: number[] = [];
    const warms: number[] = [];
    const v = new THREE.Vector3();
    const nrm = new THREE.Vector3();
    let seed = 1;
    const rnd = () => {
      seed = (seed * 16807) % 2147483647;
      return seed / 2147483647;
    };
    for (const mesh of [hemiL, hemiR, cerebellum]) {
      const g = mesh.geometry;
      const pos = g.getAttribute("position") as THREE.BufferAttribute;
      const nor = g.getAttribute("normal") as THREE.BufferAttribute;
      const fold = g.getAttribute("aFold") as THREE.BufferAttribute;
      const every = mesh === cerebellum ? 70 : 38;
      for (let i = 0; i < pos.count; i++) {
        if (fold.getX(i) > 0.1 || rnd() * every > 1) continue;
        v.fromBufferAttribute(pos, i);
        nrm.fromBufferAttribute(nor, i);
        v.addScaledVector(nrm, 0.012);
        pts.push(v.x, v.y, v.z);
        seeds.push(rnd());
        warms.push(rnd() < 0.36 ? 1 : 0);
      }
    }
    const sg = new THREE.BufferGeometry();
    sg.setAttribute("position", new THREE.Float32BufferAttribute(pts, 3));
    sg.setAttribute("aSeed", new THREE.Float32BufferAttribute(seeds, 1));
    sg.setAttribute("aWarm", new THREE.Float32BufferAttribute(warms, 1));
    sparks = new THREE.Points(sg, sparkMaterial);
    brain.add(sparks);

    // Snap each module's anchor onto the surface.
    const target = new THREE.Vector3();
    for (const id of Object.keys(ANCHOR_TARGETS) as ModuleId[]) {
      target.set(...ANCHOR_TARGETS[id]);
      let best = Infinity;
      const bp = new THREE.Vector3();
      const bn = new THREE.Vector3();
      for (const mesh of [hemiL, hemiR, cerebellum, stem]) {
        const pos = mesh.geometry.getAttribute("position") as THREE.BufferAttribute;
        const nor = mesh.geometry.getAttribute("normal") as THREE.BufferAttribute;
        for (let i = 0; i < pos.count; i += 3) {
          v.fromBufferAttribute(pos, i);
          const dd = v.distanceToSquared(target);
          if (dd < best) {
            best = dd;
            bp.copy(v);
            bn.fromBufferAttribute(nor, i);
          }
        }
      }
      anchors[id] = { p: bp, n: bn.normalize() };
    }
  };

  const renderPass = new RenderPass(scene, camera);
  const bloom = new UnrealBloomPass(new THREE.Vector2(256, 256), 0.8, 0.45, 0.55);
  const composer = new EffectComposer(renderer);
  composer.addPass(renderPass);
  composer.addPass(bloom);
  const floor = new ShaderPass({
    uniforms: { tDiffuse: { value: null }, uPage: shared.uPage, uCut: { value: 0.02 } },
    vertexShader: FLOOR_VS,
    fragmentShader: FLOOR_FS,
  });
  composer.addPass(floor);
  composer.addPass(new OutputPass());

  let W = 1;
  let H = 1;
  let focus: FocusBox = { x: 0, y: 0, w: 1, h: 1 };
  let dark = true;

  const state = {
    hoverAmt: 0,
    dimAmt: 0,
    hoverRegion: 0,
    sparks: 1,
    waveTime: -1,
    nextWave: 3,
    camTarget: new THREE.Vector3(0, CENTER_Y, 0),
  };

  const raycaster = new THREE.Raycaster();
  const ndc = new THREE.Vector2();
  const tmp = new THREE.Vector3();
  const tmpN = new THREE.Vector3();
  const colorByModule = {} as Record<ModuleId, THREE.Color>;

  const placeCamera = (zoom: number, leave: BrainFrame["leave"]) => {
    const aspect = W / H;
    // Fit the brain into the focus box, not the whole canvas: the canvas
    // spans the hub, the brain only its middle.
    const t = Math.tan(THREE.MathUtils.degToRad(FOV / 2));
    const byHeight = (HALF_HEIGHT / t) * (H / focus.h);
    const byWidth = (1.62 / (t * aspect)) * (W / focus.w);
    let dist = Math.max(byHeight, byWidth) / zoom;

    const target = new THREE.Vector3(0, CENTER_Y, 0);
    if (leave) {
      const a = anchors[leave.id];
      if (a) {
        const world = brain.localToWorld(a.p.clone());
        target.lerp(world, leave.progress);
        dist *= 1 - 0.5 * leave.progress;
      }
    }
    state.camTarget.copy(target);

    // From the front and a little above: both hemispheres side by side,
    // the fissure running down the middle.
    const az = 0;
    const el = 0.4;
    camera.position.set(
      target.x + dist * Math.sin(az) * Math.cos(el),
      target.y + dist * Math.sin(el),
      target.z + dist * Math.cos(az) * Math.cos(el),
    );
    camera.lookAt(target);
    camera.aspect = aspect;
    // Shift the picture so its center lands on the focus box's center.
    camera.setViewOffset(W, H, W / 2 - (focus.x + focus.w / 2), H / 2 - (focus.y + focus.h / 2), W, H);
    camera.updateProjectionMatrix();
  };

  // A look's own colors win over the theme's; whatever it leaves out comes
  // from the page's CSS.
  function applyLook() {
    shared.uLineWidth.value = look.lineWidth;
    shared.uEmit.value = look.emit;
    shared.uWarmAmt.value = look.warm;
    shared.uFresnelAmt.value = look.fresnel;
    shared.uTissueAmt.value = look.tissue;
    shared.uScan.value = look.scan;
    shared.uFlow.value = look.flow;
    shared.uGradient.value = look.gradient;
    platformMaterial.uniforms.uRing.value = look.ring;
    if (!themeColors) return;
    const glow = look.colors.glow ?? themeColors.glow;
    const spark = look.colors.spark ?? themeColors.spark;
    shared.uGlowColor.value.setStyle(glow);
    shared.uGlow2.value.setStyle(look.colors.glow2 ?? glow);
    shared.uTissueColor.value.setStyle(look.colors.tissue ?? themeColors.tissue);
    shared.uSparkColor.value.setStyle(spark);
    warmFlareMaterial.color.setStyle(spark);
  }
  applyLook();

  return {
    resize(width, height, dpr, box) {
      focus = box;
      // Scrolling moves the focus box without resizing anything; skip the
      // render-target reallocation then.
      if (Math.max(1, width) === W && Math.max(1, height) === H && dpr === renderer.getPixelRatio()) return;
      W = Math.max(1, width);
      H = Math.max(1, height);
      focus = box;
      renderer.setPixelRatio(dpr);
      renderer.setSize(W, H, false);
      composer.setPixelRatio(dpr);
      composer.setSize(W, H);
    },

    setColors(c) {
      dark = c.dark;
      themeColors = c;
      applyLook();
      shared.uPage.value.setStyle(c.page);
      shared.uDark.value = c.dark ? 1 : 0;
      floor.uniforms.uCut.value = c.dark ? 0.02 : 0;
      renderer.setClearColor(new THREE.Color(c.page), 1);
      for (const id of Object.keys(c.modules) as ModuleId[]) {
        colorByModule[id] = new THREE.Color(c.modules[id]);
      }
    },

    setLook(next) {
      look = next;
      applyLook();
      shared.uLineFreq.value = 7.2 * density * look.lineFreq;
    },

    rebuild,

    render(f) {
      const time = f.reduce ? 4 : f.t * f.settings.speed;
      shared.uTime.value = time;
      const beatPhase = time % 3.2;
      shared.uBeat.value = f.reduce ? 0 : Math.exp(-beatPhase * 7) + 0.55 * Math.exp(-Math.max(0, beatPhase - 0.19) * 7);

      const k = f.reduce ? 1 : 1 - Math.exp(-f.dt * 7);
      const focusId = f.leave ? f.leave.id : f.hover;
      if (focusId) {
        state.hoverRegion = REGION_INDEX[focusId];
        const target = colorByModule[focusId];
        if (target) shared.uHoverColor.value.lerp(target, state.hoverAmt < 0.05 ? 1 : k);
      }
      state.hoverAmt += ((focusId ? 1 : 0) - state.hoverAmt) * k;
      state.dimAmt += ((focusId ? 1 : 0) - state.dimAmt) * k;
      shared.uHoverRegion.value = state.hoverAmt > 0.01 ? state.hoverRegion : 0;
      shared.uHoverAmt.value = state.hoverAmt;
      shared.uDimAmt.value = state.dimAmt;

      shared.uBoot.value = f.boot;
      shared.uGlow.value = f.settings.glow;
      shared.uFade.value = f.leave ? 1 - smooth(0.45, 1, f.leave.progress) * 0.92 : 1;
      state.sparks += ((f.settings.sparks ? 1 : 0) - state.sparks) * k;
      sparkMaterial.uniforms.uSparks.value = state.sparks;
      // Point size is in device pixels and falls off with depth in the
      // shader; tying it to canvas height keeps sparks the same size
      // relative to the brain at any window size.
      sparkMaterial.uniforms.uSize.value = 0.085 * H * renderer.getPixelRatio() * f.settings.scale;
      platformMaterial.uniforms.uAppear.value = smooth(0, 0.45, f.boot);
      const flareOn = smooth(0.45, 1, f.boot) * shared.uFade.value * Math.max(0.3, f.settings.glow);
      flareMaterial.opacity = Math.min(1, flareOn * (0.7 + 0.3 * shared.uBeat.value) * look.flare);
      warmFlareMaterial.opacity = Math.min(1, flareOn * (0.55 + 0.45 * shared.uBeat.value) * look.flare);
      flare.scale.setScalar(0.62 + 0.08 * shared.uBeat.value);
      warmFlare.scale.setScalar(0.26 + 0.06 * shared.uBeat.value);

      // Waves: answer each hover, and think on their own every few seconds.
      if (state.waveTime >= 0) state.waveTime += f.dt * f.settings.speed;
      if (state.waveTime > 1.8) state.waveTime = -1;
      if (!f.reduce && f.boot >= 1 && !f.leave && f.t > state.nextWave) {
        shared.uWaveOrigin.value.copy(CORE);
        state.waveTime = 0;
        state.nextWave = f.t + (4 + Math.random() * 3) / f.settings.speed;
      }
      shared.uWaveTime.value = state.waveTime;

      brain.scale.setScalar(f.settings.scale);
      const sway = f.reduce ? 0 : Math.sin(f.t * 0.21) * 0.22;
      brain.rotation.set(f.pitch, f.yaw + sway, 0, "YXZ");
      brain.position.y = f.reduce ? 0 : Math.sin(f.t * 0.7) * 0.018;

      bloom.strength = (dark ? look.bloom : 0.25) * f.settings.glow;
      placeCamera(f.zoom, f.leave);
      composer.render(f.dt);
    },

    project(id) {
      const a = anchors[id];
      if (!a) return null;
      brain.updateMatrixWorld();
      const world = brain.localToWorld(tmp.copy(a.p));
      const normal = tmpN.copy(a.n).applyQuaternion(brain.quaternion);
      const facing = Math.max(0, normal.dot(camera.position.clone().sub(world).normalize()));
      world.project(camera);
      return { x: ((world.x + 1) / 2) * W, y: ((1 - world.y) / 2) * H, facing };
    },

    hit(x, y) {
      const meshes = [hemiL, hemiR, cerebellum, stem].filter((m): m is THREE.Mesh => m !== null);
      if (!meshes.length) return null;
      ndc.set((x / W) * 2 - 1, -((y / H) * 2 - 1));
      raycaster.setFromCamera(ndc, camera);
      const hitResult = raycaster.intersectObjects(meshes, false)[0];
      if (!hitResult) return null;
      const mesh = hitResult.object as THREE.Mesh;
      const fixed = (mesh.material as THREE.ShaderMaterial).uniforms.uFixedRegion.value as number;
      if (fixed > 0) return MODULE_BY_INDEX[fixed];
      return MODULE_BY_INDEX[regionOf(brain.worldToLocal(hitResult.point.clone()))];
    },

    wave(id) {
      if (!id) return;
      const a = anchors[id];
      shared.uWaveOrigin.value.copy(a ? a.p : CORE);
      state.waveTime = 0;
    },

    dispose() {
      disposeBrain();
      flareTexture.dispose();
      flareMaterial.dispose();
      warmFlareMaterial.dispose();
      platform.geometry.dispose();
      platformMaterial.dispose();
      sparkMaterial.dispose();
      for (const m of Object.values(materials)) m.dispose();
      composer.dispose();
      renderer.dispose();
    },
  };
}
