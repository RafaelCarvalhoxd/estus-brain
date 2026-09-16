"use client";

import { useEffect, useRef, useState, useSyncExternalStore, type CSSProperties } from "react";
import Link from "next/link";
import { IconClose, IconGear } from "@/components/icons";
import { ThemeToggle } from "@/components/ThemeToggle";
import { LOOK_LIST } from "./looks";
import {
  SETTING_RANGE,
  getServerSettings,
  getSettings,
  hydrateSettings,
  resetSettings,
  setSettings,
  subscribeSettings,
  type BrainSettings,
} from "./settings";

type NumericKey = keyof typeof SETTING_RANGE;

const ROWS: { key: NumericKey; label: string; format: (v: number) => string }[] = [
  { key: "density", label: "Dobras", format: (v) => `${Math.round(v * 100)}%` },
  { key: "glow", label: "Brilho", format: (v) => `${Math.round(v * 100)}%` },
  { key: "speed", label: "Sinais", format: (v) => `${Math.round(v * 100)}%` },
  { key: "scale", label: "Tamanho", format: (v) => `${Math.round(v * 100)}%` },
  { key: "depth", label: "Relevo", format: (v) => `${Math.round((v / 180) * 100)}%` },
];

export function BrainControls() {
  const settings = useSyncExternalStore(subscribeSettings, getSettings, getServerSettings);
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    hydrateSettings();
  }, []);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onDown = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onDown);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onDown);
    };
  }, [open]);

  return (
    <div className="brain-controls" ref={wrapRef}>
      <button
        type="button"
        className={`brain-controls-btn${open ? " is-open" : ""}`}
        aria-expanded={open}
        aria-label="Configurações"
        title="Configurações"
        onClick={() => setOpen((v) => !v)}
      >
        <IconGear />
      </button>

      {open && (
        <div className="brain-panel" role="dialog" aria-label="Configurações">
          <div className="brain-panel-head">
            <b>Configurações</b>
            <button type="button" className="icon-btn" aria-label="Fechar" onClick={() => setOpen(false)}>
              <IconClose />
            </button>
          </div>

          <div className="brain-theme">
            <p className="brain-section-title">Tema</p>
            <ThemeToggle />
          </div>

          <p className="brain-section-title">Modelo do cérebro</p>
          <div className="look-options" role="radiogroup" aria-label="Modelo do cérebro">
            {LOOK_LIST.map((look) => {
              const on = settings.look === look.id;
              const glow = look.colors.glow ?? "var(--brain-glow)";
              const glow2 = look.colors.glow2 ?? glow;
              return (
                <button
                  key={look.id}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  className={`look-option${on ? " is-on" : ""}`}
                  title={look.description}
                  onClick={() => setSettings({ look: look.id })}
                >
                  <span
                    className="look-swatch"
                    aria-hidden="true"
                    style={{ "--swatch-a": glow, "--swatch-b": glow2 } as CSSProperties}
                  />
                  {look.name}
                </button>
              );
            })}
          </div>
          <Link href="/cerebro" className="brain-compare">
            Comparar os modelos ao vivo
          </Link>

          <p className="brain-section-title">Ajustes</p>
          {ROWS.map(({ key, label, format }) => (
            <label className="brain-row" key={key}>
              <span>{label}</span>
              <input
                type="range"
                min={SETTING_RANGE[key].min}
                max={SETTING_RANGE[key].max}
                step={SETTING_RANGE[key].step}
                value={settings[key]}
                onChange={(e) => setSettings({ [key]: Number(e.target.value) } as Partial<BrainSettings>)}
              />
              <em>{format(settings[key])}</em>
            </label>
          ))}

          <div className="brain-row brain-row-switch">
            <span>Faíscas entre lobos</span>
            <button
              type="button"
              role="switch"
              aria-checked={settings.sparks}
              aria-label="Faíscas entre lobos"
              className={`switch${settings.sparks ? " on" : ""}`}
              onClick={() => setSettings({ sparks: !settings.sparks })}
            />
          </div>

          <div className="brain-panel-foot">
            <span>Arraste o cérebro pra girar, role pra aproximar</span>
            <button type="button" className="btn-text" onClick={resetSettings}>
              Restaurar padrão
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
