import type { CSSProperties, ReactNode } from "react";
import Link from "next/link";
import { MODULE_META, type ModuleId } from "@/lib/modules";
import { IconBrain, IconChevronRight } from "./icons";
import { ThemeToggle } from "./ThemeToggle";
import { BrainBackdrop } from "./brain/BrainBackdrop";

// Modules are only ever entered from the núcleo, so there is no nav here —
// just the way back, and which module you're in. It also puts the brain
// behind the screen, with this module's region lit. A screen inside a module
// (one board, say) passes itself as children: it becomes the last crumb and
// the module name turns into the way back to the module.
export function ModuleTopBar({ module, children }: { module: ModuleId; children?: ReactNode }) {
  const mod = MODULE_META[module];

  return (
    <>
      <BrainBackdrop module={module} />
      <header className="modbar" style={{ "--mod": `var(${mod.colorVar})` } as CSSProperties}>
        <div className="modbar-crumbs">
          <Link href="/" className="modbar-home" aria-label="Voltar ao núcleo">
            <IconBrain />
          </Link>
          <Link href="/" className="modbar-back">
            Núcleo
          </Link>
          <IconChevronRight />
          {children ? (
            <>
              <Link href={mod.href} className="modbar-current modbar-module">
                {mod.label}
              </Link>
              <IconChevronRight />
              {children}
            </>
          ) : (
            <span className="modbar-current" aria-current="page">
              {mod.label}
            </span>
          )}
        </div>
        <ThemeToggle />
      </header>
    </>
  );
}
