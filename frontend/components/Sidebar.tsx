import Link from "next/link";
import {
  IconOverview,
  IconList,
  IconCard,
  IconTag,
  IconRepeat,
  IconLock,
} from "./icons";

export function Sidebar() {
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
        <Link className="active" href="/">
          <IconOverview />
          Visão geral
        </Link>
        <a href="#">
          <IconList />
          Lançamentos
        </a>
        <a href="#">
          <IconCard />
          Cartões
        </a>
        <a href="#">
          <IconTag />
          Categorias
        </a>
        <a href="#">
          <IconRepeat />
          Recorrentes
        </a>
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
