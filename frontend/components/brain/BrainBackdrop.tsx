"use client";

import { useEffect, useRef, useSyncExternalStore } from "react";
import type { ModuleId } from "@/lib/modules";
import { createBrain3D, type Brain3D } from "./brain3d";
import { LOOKS } from "./looks";
import { getServerSettings, getSettings, hydrateSettings, subscribeSettings } from "./settings";
import { readBrainColors } from "./theme";

// The brain behind every module screen: dim, out of focus, turning slowly,
// with the module's own region lit in its color (none for screens that
// belong to the whole brain, like the chat) — so each screen still reads
// as a part of the same brain. It follows the model and knobs chosen in the
// núcleo's settings, draws at half frame rate and at 1× pixel density (it's
// blurred anyway), and stops while the tab is hidden.
export function BrainBackdrop({ module }: { module: ModuleId | null }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const engineRef = useRef<Brain3D | null>(null);
  const settings = useSyncExternalStore(subscribeSettings, getSettings, getServerSettings);
  const settingsRef = useRef(settings);

  useEffect(() => {
    hydrateSettings();
  }, []);

  useEffect(() => {
    settingsRef.current = settings;
  }, [settings]);

  useEffect(() => {
    engineRef.current?.setLook(LOOKS[settings.look]);
  }, [settings.look]);

  useEffect(() => {
    engineRef.current?.rebuild(settings.density, settings.depth);
  }, [settings.density, settings.depth]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const engine = createBrain3D(canvas, LOOKS[settingsRef.current.look]);
    if (!engine) return;
    engineRef.current = engine;
    engine.rebuild(settingsRef.current.density, settingsRef.current.depth);
    engine.setColors(readBrainColors());

    const themeObserver = new MutationObserver(() => engine.setColors(readBrainColors()));
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

    // The brain is fitted to this box: a smaller box means a smaller brain.
    // Centered horizontally, low on the screen, like the brain settled behind
    // the overview.
    const size = () => {
      const w = window.innerWidth;
      const h = window.innerHeight;
      engine.resize(w, h, 1, { x: w * 0.2, y: h * 0.36, w: w * 0.6, h: h * 0.62 });
    };
    size();
    window.addEventListener("resize", size);

    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const t0 = performance.now();
    let last = t0;
    let lastDraw = 0;
    let raf = 0;
    const frame = (now: number) => {
      raf = requestAnimationFrame(frame);
      if (document.hidden) return;
      if (now - lastDraw < 33) return;
      if (reduce && lastDraw > 0 && now - t0 > 1500) return;
      const dt = Math.min(0.1, (now - last) / 1000);
      last = now;
      lastDraw = now;
      const t = (now - t0) / 1000;
      engine.render({
        t,
        dt,
        yaw: reduce ? 0.2 : Math.sin(t * 0.1) * 0.55,
        pitch: 0.12,
        zoom: 1,
        hover: module,
        leave: null,
        boot: reduce ? 1 : Math.min(1, t / 1.2),
        settings: settingsRef.current,
        reduce,
      });
    };
    raf = requestAnimationFrame(frame);

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", size);
      themeObserver.disconnect();
      engineRef.current = null;
      engine.dispose();
    };
  }, [module]);

  return <canvas ref={canvasRef} className="brain-backdrop" aria-hidden="true" />;
}
