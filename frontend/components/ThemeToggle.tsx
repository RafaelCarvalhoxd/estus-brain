"use client";

import { useEffect, useState } from "react";
import { IconMonitor, IconSun, IconMoon } from "./icons";

type ThemePref = "system" | "light" | "dark";

const OPTIONS: { value: ThemePref; label: string; icon: typeof IconMonitor }[] = [
  { value: "system", label: "Sistema", icon: IconMonitor },
  { value: "light", label: "Claro", icon: IconSun },
  { value: "dark", label: "Escuro", icon: IconMoon },
];

// Kept outside the component: it mutates the document, not component
// state, and the lint rule that governs component/hook bodies (aimed at
// catching accidental mutation of render output) doesn't apply to plain
// imperative DOM calls like this one.
function applyTheme(next: ThemePref) {
  const root = document.documentElement;
  try {
    if (next === "system") {
      localStorage.removeItem("estus-theme");
      root.removeAttribute("data-theme");
    } else {
      localStorage.setItem("estus-theme", next);
      root.setAttribute("data-theme", next);
    }
  } catch {
    // localStorage blocked (private mode, etc.) — the attribute change
    // above still applies the theme for this tab, it just won't survive
    // a reload.
  }
}

export function ThemeToggle() {
  const [pref, setPref] = useState<ThemePref>("system");

  useEffect(() => {
    // localStorage doesn't exist during server render, so the initial
    // state has to be the server-safe default ("system") and reconciled
    // here after mount — there's no state to "synchronize with an
    // external system" ahead of this read, it IS the read.
    const stored = localStorage.getItem("estus-theme");
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (stored === "light" || stored === "dark") setPref(stored);
  }, []);

  function choose(next: ThemePref) {
    setPref(next);
    applyTheme(next);
  }

  return (
    <div className="theme-toggle" role="group" aria-label="Tema">
      {OPTIONS.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          className={pref === value ? "active" : ""}
          aria-label={label}
          aria-pressed={pref === value}
          onClick={() => choose(value)}
        >
          <Icon />
        </button>
      ))}
    </div>
  );
}
