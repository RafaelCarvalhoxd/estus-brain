"use client";

import { useEffect, useRef, useSyncExternalStore } from "react";
import { createBrain3D } from "./brain3d";
import { LOOK_LIST, type BrainLook } from "./looks";
import { getServerSettings, getSettings, hydrateSettings, setSettings, subscribeSettings } from "./settings";
import { readBrainColors } from "./theme";

// A live, slowly turning brain in one look. Each preview is its own WebGL
// scene; the ones scrolled out of view stop drawing.
function LookPreview({ look }: { look: BrainLook }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const engine = createBrain3D(canvas, look);
    if (!engine) return;
    engine.rebuild(1, 65);
    engine.setColors(readBrainColors());

    const themeObserver = new MutationObserver(() => engine.setColors(readBrainColors()));
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

    const size = () => {
      const r = canvas.getBoundingClientRect();
      engine.resize(r.width, r.height, Math.min(2, window.devicePixelRatio || 1), {
        x: 0,
        y: 0,
        w: Math.max(1, r.width),
        h: Math.max(1, r.height),
      });
    };
    const ro = new ResizeObserver(size);
    ro.observe(canvas);
    size();

    let visible = true;
    const io = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
    });
    io.observe(canvas);

    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const t0 = performance.now();
    let last = t0;
    let raf = 0;
    const frame = (now: number) => {
      raf = requestAnimationFrame(frame);
      const dt = Math.min(0.05, (now - last) / 1000);
      last = now;
      if (!visible) return;
      const t = (now - t0) / 1000;
      engine.render({
        t,
        dt,
        yaw: reduce ? 0.35 : Math.sin(t * 0.35) * 0.5,
        pitch: 0.04,
        zoom: 1,
        hover: null,
        leave: null,
        boot: reduce ? 1 : Math.min(1, t / 1.6),
        settings: { glow: 1, speed: 1, scale: 1.05, sparks: true },
        reduce,
      });
    };
    raf = requestAnimationFrame(frame);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      io.disconnect();
      themeObserver.disconnect();
      engine.dispose();
    };
  }, [look]);

  return <canvas ref={canvasRef} aria-hidden="true" />;
}

export function LookPicker() {
  const settings = useSyncExternalStore(subscribeSettings, getSettings, getServerSettings);

  useEffect(() => {
    hydrateSettings();
  }, []);

  return (
    <div className="looks-grid">
      {LOOK_LIST.map((look) => {
        const active = settings.look === look.id;
        const choose = () => setSettings({ look: look.id });
        return (
          <article key={look.id} className={`look-card${active ? " is-active" : ""}`}>
            <button type="button" className="look-stage" onClick={choose} aria-label={`Usar o estilo ${look.name}`}>
              <LookPreview look={look} />
              {active && <span className="look-badge">Em uso</span>}
            </button>
            <div className="look-info">
              <div>
                <h2>{look.name}</h2>
                <p>{look.description}</p>
              </div>
              <button
                type="button"
                className={active ? "btn-outline" : "btn-primary"}
                onClick={choose}
                disabled={active}
              >
                {active ? "Em uso" : "Usar este"}
              </button>
            </div>
          </article>
        );
      })}
    </div>
  );
}
