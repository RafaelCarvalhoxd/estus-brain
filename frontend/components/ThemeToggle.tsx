"use client";

import { useEffect, useState } from "react";
import { IconSun, IconMoon } from "./icons";

type ThemePref = "dark" | "light";

const OPTIONS: { value: ThemePref; label: string; icon: typeof IconMoon }[] = [
  { value: "dark", label: "Escuro", icon: IconMoon },
  { value: "light", label: "Claro", icon: IconSun },
];

// Kept outside the component: it mutates the document, not component
// state. Dark is the unmarked default, so choosing it clears the
// attribute and the stored choice rather than writing "dark".
function applyTheme(next: ThemePref) {
  const root = document.documentElement;
  if (next === "dark") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", "light");
  try {
    if (next === "dark") localStorage.removeItem("estus-theme");
    else localStorage.setItem("estus-theme", "light");
  } catch {
    // localStorage blocked (private mode, etc.) — the attribute change
    // above still applies the theme for this tab, it just won't survive
    // a reload.
  }
}

export function ThemeToggle() {
  const [pref, setPref] = useState<ThemePref>("dark");

  useEffect(() => {
    // The attribute set by the layout's pre-paint script is the source of
    // truth; the server can't know it, so reconcile after mount.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (document.documentElement.dataset.theme === "light") setPref("light");
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
          title={label}
          aria-pressed={pref === value}
          onClick={() => choose(value)}
        >
          <Icon />
        </button>
      ))}
    </div>
  );
}
