"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const TABS = [
  { href: "/financeiro", label: "Dashboard" },
  { href: "/financeiro/lancamentos", label: "Lançamentos" },
  { href: "/financeiro/categorias", label: "Categorias" },
  { href: "/financeiro/cartoes", label: "Cartões" },
  { href: "/financeiro/contas", label: "Contas" },
];

export function FinanceTabs() {
  const pathname = usePathname();

  return (
    <nav className="module-tabs">
      {TABS.map((tab) => (
        <Link
          key={tab.href}
          href={tab.href}
          className={pathname === tab.href ? "module-tab active" : "module-tab"}
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  );
}
