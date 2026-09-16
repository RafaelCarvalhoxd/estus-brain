import type { CSSProperties } from "react";
import Link from "next/link";
import { LookPicker } from "@/components/brain/LookPicker";
import { ThemeToggle } from "@/components/ThemeToggle";
import { IconBrain, IconChevronRight } from "@/components/icons";
import "../ui.css";
import "./looks.css";

// Not a module: a side page for choosing how the núcleo's brain is drawn.
// Reached from the brain's settings panel; the choice applies on the home
// as soon as it's made.
export default function CerebroPage() {
  return (
    <div className="looks-page">
      <header className="modbar" style={{ "--mod": "var(--brain-glow)" } as CSSProperties}>
        <div className="modbar-crumbs">
          <Link href="/" className="modbar-home" aria-label="Voltar ao núcleo">
            <IconBrain />
          </Link>
          <Link href="/" className="modbar-back">
            Núcleo
          </Link>
          <IconChevronRight />
          <span className="modbar-current" aria-current="page">
            Estilo do cérebro
          </span>
        </div>
        <ThemeToggle />
      </header>

      <main className="looks-main">
        <div className="looks-head">
          <h1 className="page-title">Estilo do cérebro</h1>
          <p>Cada prévia está rodando de verdade. Escolha uma e ela passa a valer na home na hora.</p>
        </div>
        <LookPicker />
      </main>
    </div>
  );
}
