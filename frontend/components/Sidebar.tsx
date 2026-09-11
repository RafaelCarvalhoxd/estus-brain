"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  IconOverview,
  IconReceipt,
  IconLock,
  IconNote,
  IconBell,
  IconCalendar,
} from "./icons";

const NAV = [
  { href: "/", label: "Visão geral", icon: IconOverview },
  { href: "/contas", label: "Contas", icon: IconReceipt },
  { href: "/senhas", label: "Senhas", icon: IconLock },
  { href: "/notas", label: "Notas", icon: IconNote },
  { href: "/lembretes", label: "Lembretes", icon: IconBell },
  { href: "/agenda", label: "Agenda", icon: IconCalendar },
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
        {NAV.map(({ href, label, icon: Icon }) => (
          <Link key={href} className={pathname === href ? "active" : ""} href={href}>
            <Icon />
            {label}
          </Link>
        ))}
      </nav>

      <div className="rail-foot">
        <IconLock />
        <span>
          <b>Acesso por certificado</b>
          <br />
          conexão validada via mTLS
        </span>
      </div>
    </aside>
  );
}
