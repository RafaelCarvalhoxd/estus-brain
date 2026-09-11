import Link from "next/link";
import { getMonthSummary } from "@/lib/api";
import { getBillSummary, listBills } from "@/lib/bills";
import { listReminders } from "@/lib/reminders";
import { listEvents } from "@/lib/agenda";
import { listNotes } from "@/lib/notes";
import { currentYearMonth } from "@/lib/month";
import { Sidebar } from "@/components/Sidebar";
import { DeltaPill } from "@/components/DeltaPill";
import "./ui.css";

function formatDay(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("pt-BR", { day: "2-digit", month: "short" });
}

export default async function HomePage() {
  const month = currentYearMonth();
  const now = new Date();
  const in14Days = new Date(now.getTime() + 14 * 24 * 60 * 60 * 1000);

  const [finance, billSummary, overdueBills, reminders, events, notes] = await Promise.all([
    getMonthSummary(month),
    getBillSummary(),
    listBills("pagar"),
    listReminders(),
    listEvents(now, in14Days),
    listNotes(),
  ]);

  const openReminders = reminders.filter((r) => !r.done).slice(0, 3);
  const upcomingEvents = events.slice(0, 3);
  const recentNotes = notes.slice(0, 3);
  const lateBills = overdueBills.filter((b) => b.status === "atrasado").length;

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Visão geral</h1>
          </div>

          <section className="digest-grid">
            <div className="panel">
              <div className="panel-head">
                <h2>Financeiro</h2>
                <Link className="btn-text" href="/financeiro">
                  Abrir
                </Link>
              </div>
              <p className="tile-label">Gasto do mês</p>
              <p className="tile-figure tab">{finance.total.formatted}</p>
              <p className="tile-sub">
                <DeltaPill currentCents={finance.total.cents} previousCents={finance.previous_month.cents} />
                vs. mês anterior
              </p>
            </div>

            <div className="panel">
              <div className="panel-head">
                <h2>Contas</h2>
                <Link className="btn-text" href="/financeiro/contas">
                  Abrir
                </Link>
              </div>
              <p className="tile-label">A pagar em aberto</p>
              <p className="tile-figure tab">{billSummary.payable_open.formatted}</p>
              {lateBills > 0 ? (
                <p className="tile-sub">
                  <span className="pill bad">{lateBills} atrasada{lateBills > 1 ? "s" : ""}</span>
                </p>
              ) : (
                <p className="tile-sub">nada atrasado</p>
              )}
            </div>

            <div className="panel">
              <div className="panel-head">
                <h2>Lembretes</h2>
                <Link className="btn-text" href="/lembretes">
                  Abrir
                </Link>
              </div>
              {openReminders.length === 0 ? (
                <p className="empty-note">Nada pendente.</p>
              ) : (
                openReminders.map((r) => (
                  <div className="digest-row" key={r.id}>
                    <span className="digest-row-title">{r.title}</span>
                    <span className="digest-row-meta">{r.due_at ? formatDay(r.due_at) : "sem data"}</span>
                  </div>
                ))
              )}
            </div>

            <div className="panel">
              <div className="panel-head">
                <h2>Agenda</h2>
                <Link className="btn-text" href="/agenda">
                  Abrir
                </Link>
              </div>
              {upcomingEvents.length === 0 ? (
                <p className="empty-note">Nada nos próximos 14 dias.</p>
              ) : (
                upcomingEvents.map((e) => (
                  <div className="digest-row" key={e.id}>
                    <span className="digest-row-title">{e.title}</span>
                    <span className="digest-row-meta">{formatDay(e.starts_at)}</span>
                  </div>
                ))
              )}
            </div>

            <div className="panel">
              <div className="panel-head">
                <h2>Notas</h2>
                <Link className="btn-text" href="/notas">
                  Abrir
                </Link>
              </div>
              {recentNotes.length === 0 ? (
                <p className="empty-note">Nenhuma nota ainda.</p>
              ) : (
                recentNotes.map((n) => (
                  <div className="digest-row" key={n.id}>
                    <span className="digest-row-title">{n.title || "Sem título"}</span>
                    <span className="digest-row-meta">{formatDay(n.updated_at)}</span>
                  </div>
                ))
              )}
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}
