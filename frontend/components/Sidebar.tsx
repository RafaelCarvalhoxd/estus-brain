"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  IconOverview,
  IconWallet,
  IconLock,
  IconNote,
  IconBell,
  IconCalendar,
} from "./icons";
import { ThemeToggle } from "./ThemeToggle";

const NAV = [
  { href: "/", label: "Visão geral", icon: IconOverview, exact: true },
  { href: "/financeiro", label: "Financeiro", icon: IconWallet, exact: false },
  { href: "/senhas", label: "Senhas", icon: IconLock, exact: true },
  { href: "/notas", label: "Notas", icon: IconNote, exact: true },
  { href: "/lembretes", label: "Lembretes", icon: IconBell, exact: true },
  { href: "/agenda", label: "Agenda", icon: IconCalendar, exact: true },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="rail">
      <div className="brand">
        <span className="brand-mark">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="11" width="18" height="10" rx="2" />
            <path d="M7 11V8a5 5 0 0 1 10 0v3" />
            <circle cx="12" cy="16" r="1.6" />
          </svg>
        </span>
        <span className="brand-name">Estus Vault</span>
      </div>

      <nav className="nav">
        {NAV.map(({ href, label, icon: Icon, exact }) => {
          const active = exact ? pathname === href : pathname === href || pathname.startsWith(`${href}/`);
          return (
            <Link key={href} className={active ? "active" : ""} href={href}>
              <Icon />
              {label}
            </Link>
          );
        })}
      </nav>

      <div className="rail-bottom">
        <ThemeToggle />
        <div className="rail-foot">
          <IconLock />
          <span>
            <b>Acesso por certificado</b>
            <br />
            conexão validada via mTLS
          </span>
        </div>
      </div>
    </aside>
  );
}
