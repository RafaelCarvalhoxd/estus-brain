import Link from "next/link";
import { getMonthSummary } from "@/lib/api";
import { getBillsReceived } from "@/lib/bills";
import { listReminders } from "@/lib/reminders";
import { listEvents } from "@/lib/agenda";
import { listNotes } from "@/lib/notes";
import { currentYearMonth, formatYearMonth } from "@/lib/month";
import { Sidebar } from "@/components/Sidebar";
import { IconBell, IconCalendar, IconNote } from "@/components/icons";
import "./ui.css";

function formatCents(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function formatSigned(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL", signDisplay: "always" });
}

function formatDay(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("pt-BR", { day: "2-digit", month: "short" });
}

export default async function HomePage() {
  const month = currentYearMonth();
  const now = new Date();
  const in14Days = new Date(now.getTime() + 14 * 24 * 60 * 60 * 1000);

  const [finance, received, reminders, events, notes] = await Promise.all([
    getMonthSummary(month),
    getBillsReceived(month),
    listReminders(),
    listEvents(now, in14Days),
    listNotes(),
  ]);

  const saidas = finance.total.cents;
  const entradas = received.cents;
  const saldo = entradas - saidas;

  const openReminders = reminders.filter((r) => !r.done).slice(0, 4);
  const upcomingEvents = events.filter((e) => new Date(e.ends_at) >= now).slice(0, 4);
  const recentNotes = notes.slice(0, 4);

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Visão geral</h1>
          </div>

          <section className="hero-balance">
            <div className="hero-balance-head">
              <p className="hero-balance-label">Saldo · {formatYearMonth(month)}</p>
              <Link className="btn-text" href="/financeiro">
                Ver financeiro
              </Link>
            </div>
            <p className={`hero-balance-figure tab ${saldo >= 0 ? "is-good" : "is-bad"}`}>{formatSigned(saldo)}</p>
            <div className="hero-balance-breakdown">
              <div className="hero-balance-stat">
                <span className="hero-balance-dot dot-good" />
                <span className="hero-balance-stat-label">Entradas</span>
                <span className="hero-balance-stat-figure tab">{formatCents(entradas)}</span>
              </div>
              <div className="hero-balance-stat">
                <span className="hero-balance-dot dot-bad" />
                <span className="hero-balance-stat-label">Saídas</span>
                <span className="hero-balance-stat-figure tab">{formatCents(saidas)}</span>
              </div>
            </div>
          </section>

          <section className="overview-grid">
            <div className="panel overview-card">
              <div className="panel-head">
                <div className="overview-card-title">
                  <span className="overview-icon">
                    <IconBell />
                  </span>
                  <h2>Lembretes</h2>
                </div>
                <Link className="btn-text" href="/lembretes">
                  Abrir
                </Link>
              </div>
              {openReminders.length === 0 ? (
                <p className="empty-note">Nada pendente.</p>
              ) : (
                <div className="overview-list">
                  {openReminders.map((r) => (
                    <div className="overview-row" key={r.id}>
                      <span className="overview-row-title">{r.title}</span>
                      <span className="overview-row-meta">{r.due_at ? formatDay(r.due_at) : "sem data"}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="panel overview-card">
              <div className="panel-head">
                <div className="overview-card-title">
                  <span className="overview-icon">
                    <IconCalendar />
                  </span>
                  <h2>Agenda</h2>
                </div>
                <Link className="btn-text" href="/agenda">
                  Abrir
                </Link>
              </div>
              {upcomingEvents.length === 0 ? (
                <p className="empty-note">Nada nos próximos 14 dias.</p>
              ) : (
                <div className="overview-list">
                  {upcomingEvents.map((e) => (
                    <div className="overview-row" key={e.id}>
                      <span className="overview-row-title">{e.title}</span>
                      <span className="overview-row-meta">{formatDay(e.starts_at)}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="panel overview-card">
              <div className="panel-head">
                <div className="overview-card-title">
                  <span className="overview-icon">
                    <IconNote />
                  </span>
                  <h2>Notas</h2>
                </div>
                <Link className="btn-text" href="/notas">
                  Abrir
                </Link>
              </div>
              {recentNotes.length === 0 ? (
                <p className="empty-note">Nenhuma nota ainda.</p>
              ) : (
                <div className="overview-list">
                  {recentNotes.map((n) => (
                    <div className="overview-row" key={n.id}>
                      <span className="overview-row-title">{n.title || "Sem título"}</span>
                      <span className="overview-row-meta">{formatDay(n.updated_at)}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}
