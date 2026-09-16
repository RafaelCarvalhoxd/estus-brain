"use client";

import {
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
  type MouseEvent,
} from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ModuleId } from "@/lib/modules";
import {
  IconBell,
  IconCalendar,
  IconDumbbell,
  IconFlow,
  IconFolder,
  IconHabit,
  IconLock,
  IconNote,
  IconUtensils,
  IconWallet,
} from "@/components/icons";
import { HomeViewContext } from "@/components/HomeView";
import { createBrain3D, type Brain3D } from "./brain3d";
import { LOOKS, type LookId } from "./looks";
import { readBrainColors } from "./theme";
import { getServerSettings, getSettings, subscribeSettings } from "./settings";

export interface HubCard {
  id: ModuleId;
  href: string;
  label: string;
  colorVar: string;
  status: string;
  alert: boolean;
  side: "left" | "right";
}

type RGB = [number, number, number];
type Leave = { id: ModuleId; t0: number };

const ICONS: Record<ModuleId, () => React.JSX.Element> = {
  financeiro: IconWallet,
  agenda: IconCalendar,
  notas: IconNote,
  lembretes: IconBell,
  senhas: IconLock,
  documentos: IconFolder,
  treino: IconDumbbell,
  dieta: IconUtensils,
  quadros: IconFlow,
  habitos: IconHabit,
};

const MODULE_IDS: ModuleId[] = ["financeiro", "agenda", "treino", "dieta", "notas", "lembretes", "documentos", "senhas", "quadros", "habitos"];
const COMPACT_QUERY = "(max-width: 900px)";
const REDUCED_QUERY = "(prefers-reduced-motion: reduce)";
const TAU = Math.PI * 2;
const LEAVE_MS = 820;
const BOOT_KEY = "estus-brain-booted";

// It's a whole brain, so it can be turned a long way — but not so far that
// the connectors end up reaching round the back of it.
const YAW_LIMIT = 1.1;
const PITCH_LIMIT = 0.45;
const ZOOM_MIN = 0.75;
const ZOOM_MAX = 2;

// How to hold the brain so a module's region faces you: [yaw, pitch]. The
// camera looks from the front, so the lobes on the sides and at the back only
// come into view when the brain turns — pointing at a module's logo turns it.
const FOCUS_VIEW: Record<ModuleId, [number, number]> = {
  financeiro: [0.35, 0.05],
  treino: [0.45, 0.3],
  notas: [0.95, -0.05],
  documentos: [0, -0.25],
  agenda: [-0.3, 0.35],
  dieta: [-0.95, 0],
  quadros: [0.35, 0.35],
  habitos: [-0.35, 0.05],
  lembretes: [-1.05, 0.1],
  senhas: [-1.05, -0.2],
};

const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v);
const clamp = (v: number, lo: number, hi: number) => (v < lo ? lo : v > hi ? hi : v);
const easeInOut = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2);
const rgba = (c: RGB, a: number) => `rgba(${c[0]},${c[1]},${c[2]},${clamp01(a).toFixed(3)})`;

function parseHex(value: string, fallback: RGB): RGB {
  const m = /^#([0-9a-f]{6})$/i.exec(value.trim());
  if (!m) return fallback;
  const n = parseInt(m[1], 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

// Soft round glow stamped along the connectors and for the dust.
function makeSprite(c: RGB): HTMLCanvasElement {
  const s = document.createElement("canvas");
  s.width = s.height = 64;
  const g = s.getContext("2d")!;
  const grad = g.createRadialGradient(32, 32, 0, 32, 32, 32);
  grad.addColorStop(0, rgba(c, 1));
  grad.addColorStop(0.16, rgba(c, 0.6));
  grad.addColorStop(0.45, rgba(c, 0.14));
  grad.addColorStop(1, rgba(c, 0));
  g.fillStyle = grad;
  g.fillRect(0, 0, 64, 64);
  return s;
}

// The engine gets its colors from readBrainColors; the 2D overlay (dust,
// connectors) needs the same colors as RGB plus a glow sprite per module.
function readTheme(look: LookId) {
  const colors = readBrainColors();
  const glowRgb = parseHex(LOOKS[look].colors.glow ?? colors.glow, [77, 141, 255]);
  const modRgb = {} as Record<ModuleId, RGB>;
  const modSprite = {} as Record<ModuleId, HTMLCanvasElement>;
  for (const id of MODULE_IDS) {
    modRgb[id] = parseHex(colors.modules[id], [138, 138, 146]);
    modSprite[id] = makeSprite(modRgb[id]);
  }
  return { colors, dark: colors.dark, glowRgb, glowSprite: makeSprite(glowRgb), modRgb, modSprite };
}

export function BrainHub({ cards }: { cards: HubCard[] }) {
  const router = useRouter();
  const settings = useSyncExternalStore(subscribeSettings, getSettings, getServerSettings);
  const hubRef = useRef<HTMLDivElement>(null);
  const brainRef = useRef<HTMLDivElement>(null);
  const glRef = useRef<HTMLCanvasElement>(null);
  const overlayRef = useRef<HTMLCanvasElement>(null);
  const engineRef = useRef<Brain3D | null>(null);
  const nodeRefs = useRef<Partial<Record<ModuleId, HTMLAnchorElement | null>>>({});
  const hoverRef = useRef<ModuleId | null>(null);
  // Set only while a logo (not the brain itself) is pointed at or focused —
  // turning the brain under a cursor that's on it would move the target away.
  const nodeHoverRef = useRef<ModuleId | null>(null);
  const leaveRef = useRef<Leave | null>(null);
  const pushTimer = useRef<number | null>(null);
  const cardsRef = useRef(cards);
  const settingsRef = useRef(settings);
  const engineRuns = useRef(0);
  // Lets the render loop react to hover changes that start in the DOM (a
  // node hovered or focused) without re-running its effect.
  const onHoverChange = useRef<(id: ModuleId | null) => void>(() => {});
  const onLookChange = useRef<() => void>(() => {});

  // When the overview opens the brain sinks behind it and the logos fall off
  // the screen (CSS transitions). Layout isn't re-measured while things are
  // moving or while the overview is open, and the connector lines stop being
  // drawn once they've fallen away; the brain itself keeps turning behind
  // the frosted cards.
  const dashOpen = useContext(HomeViewContext);
  const hiddenRef = useRef(dashOpen);
  const viewChangedAt = useRef(-Infinity);
  const remeasure = useRef<() => void>(() => {});
  useEffect(() => {
    if (hiddenRef.current === dashOpen) return;
    hiddenRef.current = dashOpen;
    viewChangedAt.current = performance.now();
    const id = window.setTimeout(() => remeasure.current(), 950);
    return () => window.clearTimeout(id);
  }, [dashOpen]);

  // Everything but fold density and relief is read off the ref each frame;
  // those two reshape the geometry, so they're debounced — dragging a slider
  // shouldn't rebuild the brain sixty times a second.
  const [build, setBuild] = useState({ density: settings.density, depth: settings.depth });

  useEffect(() => {
    cardsRef.current = cards;
  }, [cards]);

  useEffect(() => {
    settingsRef.current = settings;
  }, [settings]);

  useEffect(() => {
    const id = window.setTimeout(() => setBuild({ density: settings.density, depth: settings.depth }), 220);
    return () => window.clearTimeout(id);
  }, [settings.density, settings.depth]);

  useEffect(() => {
    engineRef.current?.rebuild(build.density, build.depth);
  }, [build]);

  useEffect(() => {
    engineRef.current?.setLook(LOOKS[settings.look]);
    onLookChange.current();
  }, [settings.look]);

  // Hover lives outside React state on purpose: it changes on every pointer
  // move across the brain, and all it touches is a data attribute and a
  // class on six links — no re-render needed.
  const setHover = useCallback(
    (id: ModuleId | null) => {
      if (leaveRef.current || hoverRef.current === id) return;
      hoverRef.current = id;
      const hub = hubRef.current;
      if (hub) {
        if (id) hub.dataset.hover = id;
        else delete hub.dataset.hover;
      }
      for (const [key, el] of Object.entries(nodeRefs.current)) el?.classList.toggle("is-hot", key === id);
      onHoverChange.current(id);
      const card = id && cardsRef.current.find((c) => c.id === id);
      if (card) router.prefetch(card.href);
    },
    [router],
  );

  const go = useCallback(
    (id: ModuleId) => {
      if (leaveRef.current) return;
      const card = cardsRef.current.find((c) => c.id === id);
      if (!card) return;
      setHover(id);
      if (window.matchMedia(REDUCED_QUERY).matches || !engineRef.current) {
        router.push(card.href);
        return;
      }
      leaveRef.current = { id, t0: performance.now() };
      if (hubRef.current) hubRef.current.dataset.leaving = id;
      pushTimer.current = window.setTimeout(() => router.push(card.href), LEAVE_MS - 200);
    },
    [router, setHover],
  );

  const onNodeClick = (e: MouseEvent<HTMLAnchorElement>, id: ModuleId) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    go(id);
  };

  useEffect(() => {
    const hub = hubRef.current;
    const box = brainRef.current;
    const gl = glRef.current;
    const overlay = overlayRef.current;
    const ctx = overlay?.getContext("2d");
    if (!hub || !box || !gl || !overlay || !ctx) return;

    leaveRef.current = null;
    delete hub.dataset.leaving;
    hub.dataset.ready = "";

    const engine = createBrain3D(gl, LOOKS[settingsRef.current.look]);
    if (!engine) {
      // No WebGL: the logos, their status and the shortcuts all still work;
      // there's just no brain between them.
      hub.dataset.boot = "none";
      return;
    }
    engineRef.current = engine;
    engine.rebuild(settingsRef.current.density, settingsRef.current.depth);

    const reduce = window.matchMedia(REDUCED_QUERY).matches;
    const compactMq = window.matchMedia(COMPACT_QUERY);

    // The full power-on plays once per browser session; coming back from a
    // module gets a quick one.
    const rerun = engineRuns.current > 0;
    engineRuns.current++;
    let firstVisit = !rerun;
    if (firstVisit) {
      try {
        firstVisit = !sessionStorage.getItem(BOOT_KEY);
        sessionStorage.setItem(BOOT_KEY, "1");
      } catch {
        // storage blocked — every visit just gets the full intro
      }
    }
    const bootDur = reduce ? 0 : firstVisit ? 2.8 : 1.1;
    hub.style.setProperty("--boot", `${bootDur}s`);
    hub.dataset.boot = reduce ? "none" : firstVisit ? "full" : "quick";

    let theme = readTheme(settingsRef.current.look);
    engine.setColors(theme.colors);
    let dirty = true;
    const themeObserver = new MutationObserver(() => {
      theme = readTheme(settingsRef.current.look);
      engine.setColors(theme.colors);
      dirty = true;
    });
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

    // ---- layout: both canvases cover the viewport; the brain is fitted to
    // its box, and each connector ends at its node's logo ----
    let W = 0;
    let H = 0;
    let dpr = 1;
    const nodePts: Partial<Record<ModuleId, { x: number; y: number }>> = {};
    const measure = () => {
      if (hiddenRef.current || performance.now() - viewChangedAt.current < 900) return;
      const vw = window.innerWidth;
      const vh = window.innerHeight;
      const nextDpr = Math.min(2, window.devicePixelRatio || 1);
      if (vw !== W || vh !== H || nextDpr !== dpr) {
        W = vw;
        H = vh;
        dpr = nextDpr;
        overlay.width = Math.max(1, Math.round(W * dpr));
        overlay.height = Math.max(1, Math.round(H * dpr));
      }
      const br = box.getBoundingClientRect();
      engine.resize(W, H, dpr, { x: br.left, y: br.top, w: Math.max(1, br.width), h: Math.max(1, br.height) });
      for (const card of cardsRef.current) {
        const icon = nodeRefs.current[card.id]?.querySelector(".node-icon");
        if (!icon) continue;
        const r = icon.getBoundingClientRect();
        nodePts[card.id] = {
          x: card.side === "left" ? r.right + 6 : r.left - 6,
          y: r.top + r.height / 2,
        };
      }
      dirty = true;
    };
    const ro = new ResizeObserver(measure);
    ro.observe(hub);
    ro.observe(box);
    window.addEventListener("resize", measure);
    // The canvases are fixed; on a scrolling (phone) layout the brain box
    // moves under them, so the brain follows it.
    window.addEventListener("scroll", measure, { passive: true });
    measure();
    remeasure.current = measure;
    const measureTimer = window.setInterval(measure, 1500);

    // ---- the viewer's hands on the brain ----
    let yaw = 0;
    let pitch = 0;
    let yawV = 0;
    let pitchV = 0;
    let userZoom = 1;
    let zoomTarget = 1;
    let clock = 0;
    let lastInteract = -99;
    const pointers = new Map<number, { x: number; y: number }>();
    let dragging = false;
    let dragDist = 0;
    let dragStart = 0;
    let pinchDist = 0;
    let pointerX = 0;
    let pointerY = 0;
    let pointerActive = false;

    const onPointerDown = (e: PointerEvent) => {
      pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
      lastInteract = clock;
      dirty = true;
      if (pointers.size === 1) {
        dragging = true;
        dragDist = 0;
        dragStart = performance.now();
        yawV = 0;
        pitchV = 0;
        box.setPointerCapture(e.pointerId);
        box.style.cursor = "grabbing";
      } else if (pointers.size === 2) {
        dragging = false;
        const [a, b] = [...pointers.values()];
        pinchDist = Math.hypot(a.x - b.x, a.y - b.y);
      }
    };

    const onPointerMove = (e: PointerEvent) => {
      const br = box.getBoundingClientRect();
      pointerX = ((e.clientX - br.left) / br.width) * 2 - 1;
      pointerY = ((e.clientY - br.top) / br.height) * 2 - 1;
      pointerActive = true;

      const prev = pointers.get(e.pointerId);
      if (prev) {
        const dx = e.clientX - prev.x;
        const dy = e.clientY - prev.y;
        prev.x = e.clientX;
        prev.y = e.clientY;
        lastInteract = clock;
        dirty = true;
        if (pointers.size >= 2) {
          const [a, b] = [...pointers.values()];
          const d = Math.hypot(a.x - b.x, a.y - b.y);
          if (pinchDist > 0) zoomTarget = clamp((zoomTarget * d) / pinchDist, ZOOM_MIN, ZOOM_MAX);
          pinchDist = d;
          return;
        }
        if (dragging) {
          dragDist += Math.hypot(dx, dy);
          yaw = clamp(yaw + dx * 0.006, -YAW_LIMIT, YAW_LIMIT);
          pitch = clamp(pitch + dy * 0.004, -PITCH_LIMIT, PITCH_LIMIT);
          yawV = yawV * 0.55 + dx * 0.006 * 0.45;
          pitchV = pitchV * 0.55 + dy * 0.004 * 0.45;
          return;
        }
      }

      if (leaveRef.current) return;
      const hit = engine.hit(e.clientX, e.clientY);
      box.style.cursor = hit ? "pointer" : "";
      setHover(hit);
    };

    const endPointer = (e: PointerEvent) => {
      pointers.delete(e.pointerId);
      if (pointers.size < 2) pinchDist = 0;
      if (pointers.size === 0) {
        dragging = false;
        box.style.cursor = hoverRef.current ? "pointer" : "";
      }
      lastInteract = clock;
      dirty = true;
    };

    const onPointerLeave = () => {
      pointerActive = false;
      if (!dragging) box.style.cursor = "";
      setHover(null);
      dirty = true;
    };

    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      zoomTarget = clamp(zoomTarget * Math.exp(-e.deltaY * 0.0006), ZOOM_MIN, ZOOM_MAX);
      lastInteract = clock;
      dirty = true;
    };

    const onClick = (e: globalThis.MouseEvent) => {
      // A drag that ends over a lobe shouldn't open it.
      if (dragDist > 6 || performance.now() - dragStart > 600) return;
      const hit = engine.hit(e.clientX, e.clientY);
      if (hit) go(hit);
    };

    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      // Not while the overview or the chat covers the brain.
      if (hiddenRef.current) return;
      const target = e.target as HTMLElement | null;
      if (target?.closest("input, textarea, select, [contenteditable]")) return;
      // 1–9, then 0 for the tenth.
      const n = e.key === "0" ? 10 : Number(e.key);
      const card = Number.isInteger(n) && n >= 1 ? cardsRef.current[n - 1] : undefined;
      if (!card) return;
      e.preventDefault();
      go(card.id);
    };

    box.addEventListener("pointerdown", onPointerDown);
    box.addEventListener("pointermove", onPointerMove);
    box.addEventListener("pointerup", endPointer);
    box.addEventListener("pointercancel", endPointer);
    box.addEventListener("pointerleave", onPointerLeave);
    box.addEventListener("wheel", onWheel, { passive: false });
    box.addEventListener("click", onClick);
    window.addEventListener("keydown", onKey);

    onLookChange.current = () => {
      theme = readTheme(settingsRef.current.look);
      dirty = true;
    };

    onHoverChange.current = (id) => {
      dirty = true;
      if (!reduce) engine.wave(id);
    };

    // Dust drifting across the screen, parallaxing against the rotation.
    const dust = Array.from({ length: 90 }, () => ({
      x: Math.random(),
      y: Math.random(),
      z: -0.4 + Math.random() * 1.6,
      r: 0.5 + Math.random() * 1.4,
      ph: Math.random() * TAU,
      bokeh: Math.random() < 0.14,
    }));

    const bezier = (p0: number, c1: number, c2: number, p3: number, t: number) => {
      const u = 1 - t;
      return u * u * u * p0 + 3 * u * u * t * c1 + 3 * u * t * t * c2 + t * t * t * p3;
    };

    const drawOverlay = (t: number, bootT: number, leaveProgress: number | null) => {
      const cfg = settingsRef.current;
      const glow = cfg.glow;
      const glowOp: GlobalCompositeOperation = theme.dark ? "lighter" : "source-over";
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, overlay.width, overlay.height);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      const fade = leaveProgress === null ? 1 : 1 - clamp01((leaveProgress - 0.4) / 0.6);

      if (glow > 0.01) {
        ctx.globalCompositeOperation = glowOp;
        ctx.fillStyle = rgba(theme.glowRgb, 1);
        for (const d of dust) {
          const x = d.x * W + d.z * Math.sin(yaw) * 40;
          const y = d.y * H + d.z * Math.sin(pitch) * 30;
          const tw = 0.5 + 0.5 * Math.sin(t * 0.7 + d.ph);
          const alpha = bootT * glow * fade * (theme.dark ? 0.14 : 0.07) * (0.35 + 0.65 * tw);
          if (d.bokeh) {
            const bs = d.r * 12;
            ctx.globalAlpha = alpha * 0.5;
            ctx.drawImage(theme.glowSprite, x - bs / 2, y - bs / 2, bs, bs);
          } else {
            ctx.globalAlpha = alpha;
            ctx.fillRect(x, y, d.r, d.r);
          }
        }
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = "source-over";
      }

      if (compactMq.matches) return;

      // Thin connectors from each logo into its part of the brain: quiet at
      // rest, lit only for the one being pointed at. Ones whose anchor has
      // turned away from the viewer fade back.
      const hover = hoverRef.current;
      // The lines grow one after another in the last 45% of the boot; the
      // stagger is shared out so the last one still finishes by the end.
      const stagger = 0.15 / Math.max(1, cardsRef.current.length - 1);
      cardsRef.current.forEach((card, idx) => {
        const pt = nodePts[card.id];
        const anchor = engine.project(card.id);
        if (!pt || !anchor) return;
        const prog = reduce ? 1 : clamp01((bootT - 0.55 - idx * stagger) / 0.3);
        if (prog <= 0) return;

        const ax = anchor.x;
        const ay = anchor.y;
        const dir = card.side === "left" ? 1 : -1;
        const span = Math.abs(ax - pt.x);
        const c1x = pt.x + dir * span * 0.45;
        const c2x = ax - dir * span * 0.35;

        const heat = hover === card.id ? 1 : 0;
        const on = hover ? heat || 0.2 : 0.55;
        const seen = 0.45 + 0.55 * anchor.facing;
        const color = theme.modRgb[card.id];

        const SEG = 44;
        const upto = Math.max(1, Math.round(SEG * prog));
        const curve = new Path2D();
        curve.moveTo(pt.x, pt.y);
        for (let q = 1; q <= upto; q++) {
          const u = q / SEG;
          curve.lineTo(bezier(pt.x, c1x, c2x, ax, u), bezier(pt.y, pt.y, ay, ay, u));
        }

        if (heat) {
          ctx.globalCompositeOperation = glowOp;
          ctx.lineWidth = 4;
          ctx.strokeStyle = rgba(color, 0.16 * seen * fade * Math.max(0.3, glow));
          ctx.stroke(curve);
          ctx.globalCompositeOperation = "source-over";
        }
        ctx.lineWidth = 1 + 0.5 * heat;
        ctx.strokeStyle = rgba(color, (0.16 + 0.3 * on + 0.35 * heat) * seen * fade);
        ctx.stroke(curve);

        if (prog >= 1) {
          ctx.globalCompositeOperation = glowOp;
          ctx.globalAlpha = (0.25 + 0.55 * heat) * seen * fade;
          ctx.drawImage(theme.modSprite[card.id], ax - 7, ay - 7, 14, 14);
          ctx.globalAlpha = 1;
          ctx.globalCompositeOperation = "source-over";
        }

        // A small packet travelling from the logo into the brain.
        if (!reduce && prog >= 1 && leaveProgress === null) {
          const phase = (t * cfg.speed * (0.3 + 0.5 * heat) + idx * 0.23) % 1.6;
          if (phase < 1) {
            const qx = bezier(pt.x, c1x, c2x, ax, phase);
            const qy = bezier(pt.y, pt.y, ay, ay, phase);
            const gs = 9 + 7 * heat;
            ctx.globalCompositeOperation = glowOp;
            ctx.globalAlpha = (0.3 + 0.25 * on + 0.35 * heat) * fade * Math.max(0.3, glow);
            ctx.drawImage(theme.modSprite[card.id], qx - gs / 2, qy - gs / 2, gs, gs);
            ctx.globalAlpha = 1;
            ctx.globalCompositeOperation = "source-over";
          }
        }
      });
    };

    const t0 = performance.now();
    let last = t0;
    let raf = 0;

    const frame = (now: number) => {
      raf = requestAnimationFrame(frame);
      const dt = Math.min(0.05, (now - last) / 1000);
      last = now;
      // Reduced motion: a still picture, redrawn only when the viewer is
      // actually doing something (turning it, zooming, hovering).
      const settling = dragging || Math.abs(yawV) > 0.0004 || Math.abs(zoomTarget - userZoom) > 0.002;
      const leave = leaveRef.current;
      if (reduce && !dirty && !settling && !leave) return;
      dirty = false;
      if (!W || !H) return;

      const t = (now - t0) / 1000;
      clock = t;

      // Rotation: dragged by hand, then coasting, then drifting back toward
      // wherever the pointer is.
      if (!dragging) {
        yaw += yawV;
        pitch += pitchV;
        const decay = Math.pow(0.92, dt * 60);
        yawV *= decay;
        pitchV *= decay;
        if (yaw > YAW_LIMIT || yaw < -YAW_LIMIT) {
          yaw = clamp(yaw, -YAW_LIMIT, YAW_LIMIT);
          yawV = 0;
        }
        if (pitch > PITCH_LIMIT || pitch < -PITCH_LIMIT) {
          pitch = clamp(pitch, -PITCH_LIMIT, PITCH_LIMIT);
          pitchV = 0;
        }
        const turnTo = leave?.id ?? nodeHoverRef.current;
        if (turnTo) {
          // Swing the chosen region round to face the viewer.
          const [fy, fp] = FOCUS_VIEW[turnTo];
          const k = reduce ? 1 : 1 - Math.exp(-dt * 3.2);
          yaw += (fy - yaw) * k;
          pitch += (fp - pitch) * k;
          yawV = 0;
          pitchV = 0;
          lastInteract = t;
          dirty = true;
        } else if (!reduce) {
          const idle = clamp01((t - lastInteract - 2.2) / 2.5);
          if (idle > 0) {
            const pull = 1 - Math.exp(-dt * 1.2 * idle);
            yaw += ((pointerActive ? pointerX * 0.3 : 0) - yaw) * pull;
            pitch += ((pointerActive ? pointerY * 0.12 : 0) - pitch) * pull;
            if (idle > 0.8) zoomTarget += (1 - zoomTarget) * (1 - Math.exp(-dt * 0.5));
          }
        }
      }
      userZoom += (zoomTarget - userZoom) * (reduce ? 1 : 1 - Math.exp(-dt * 9));

      const bootT = bootDur ? clamp01(t / bootDur) : 1;
      const leaveProgress = leave ? easeInOut(clamp01((now - leave.t0) / LEAVE_MS)) : null;

      engine.render({
        t,
        dt,
        yaw,
        pitch,
        zoom: userZoom,
        hover: hoverRef.current,
        leave: leave && leaveProgress !== null ? { id: leave.id, progress: leaveProgress } : null,
        boot: bootT,
        settings: settingsRef.current,
        reduce,
      });
      if (!hiddenRef.current || now - viewChangedAt.current < 900) drawOverlay(t, bootT, leaveProgress);
    };
    raf = requestAnimationFrame(frame);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      window.removeEventListener("resize", measure);
      window.removeEventListener("scroll", measure);
      window.clearInterval(measureTimer);
      themeObserver.disconnect();
      box.removeEventListener("pointerdown", onPointerDown);
      box.removeEventListener("pointermove", onPointerMove);
      box.removeEventListener("pointerup", endPointer);
      box.removeEventListener("pointercancel", endPointer);
      box.removeEventListener("pointerleave", onPointerLeave);
      box.removeEventListener("wheel", onWheel);
      box.removeEventListener("click", onClick);
      window.removeEventListener("keydown", onKey);
      onHoverChange.current = () => {};
      onLookChange.current = () => {};
      remeasure.current = () => {};
      if (pushTimer.current) window.clearTimeout(pushTimer.current);
      engineRef.current = null;
      engine.dispose();
    };
  }, [go, setHover]);

  const renderNode = (card: HubCard) => {
    const i = cards.indexOf(card);
    const Icon = ICONS[card.id];
    // The nodes on each side follow an arc around the brain: the middle ones
    // sit further out, however many a column holds.
    const column = cards.filter((c) => c.side === card.side);
    const t = (column.indexOf(card) + 0.5) / column.length;
    const arc = Math.round(74 * (Math.sin(Math.PI * t) - Math.sin(Math.PI / (2 * column.length))));
    return (
      <Link
        key={card.id}
        href={card.href}
        ref={(el) => {
          nodeRefs.current[card.id] = el;
        }}
        className="node"
        data-side={card.side}
        style={{ "--mod": `var(${card.colorVar})`, "--i": i, "--arc": `${arc}px` } as CSSProperties}
        aria-keyshortcuts={String((i + 1) % 10)}
        aria-label={`${card.label}: ${card.status}`}
        onPointerEnter={() => {
          nodeHoverRef.current = card.id;
          setHover(card.id);
        }}
        onPointerLeave={() => {
          nodeHoverRef.current = null;
          setHover(null);
        }}
        onFocus={() => {
          nodeHoverRef.current = card.id;
          setHover(card.id);
        }}
        onBlur={() => {
          nodeHoverRef.current = null;
          setHover(null);
        }}
        onClick={(e) => onNodeClick(e, card.id)}
      >
        <span className="node-icon" aria-hidden="true">
          <Icon />
          {card.alert && <span className="node-alert" />}
        </span>
        <span className="node-text" aria-hidden="true">
          <span className="node-label">{card.label}</span>
          <span className={`node-status${card.alert ? " is-alert" : ""}`}>{card.status}</span>
        </span>
      </Link>
    );
  };

  return (
    <div className="hub" ref={hubRef}>
      <canvas ref={glRef} className="hub-gl" aria-hidden="true" />
      <canvas ref={overlayRef} className="hub-canvas" aria-hidden="true" />
      <div className="hub-col hub-col-left">{cards.filter((c) => c.side === "left").map(renderNode)}</div>
      <div className="hub-brain" ref={brainRef} />
      <div className="hub-col hub-col-right">{cards.filter((c) => c.side === "right").map(renderNode)}</div>
    </div>
  );
}
