import { googleStatus, listEvents, type Event } from "@/lib/agenda";
import { Sidebar } from "@/components/Sidebar";
import { EventRow, NewEventForm, GoogleSyncButton } from "@/components/EventForm";
import "../ui.css";
import "./agenda.css";

function dayKey(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function dayHeading(iso: string): string {
  const d = new Date(iso);
  const label = d.toLocaleDateString("pt-BR", { weekday: "long", day: "2-digit", month: "long" });
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function groupByDay(events: Event[]): { key: string; heading: string; events: Event[] }[] {
  const groups = new Map<string, Event[]>();
  for (const e of events) {
    const key = dayKey(e.starts_at);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(e);
  }
  return Array.from(groups.entries())
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, dayEvents]) => ({ key, heading: dayHeading(dayEvents[0].starts_at), events: dayEvents }));
}

export default async function AgendaPage() {
  const now = new Date();
  const from = new Date(now);
  from.setDate(from.getDate() - 7);
  const to = new Date(now);
  to.setDate(to.getDate() + 30);

  const [events, google] = await Promise.all([listEvents(from, to), googleStatus()]);
  const upcoming = events.filter((e) => new Date(e.ends_at) >= now);
  const days = groupByDay(upcoming);

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Agenda</h1>
            <div className="agenda-google-status">
              {google.connected ? (
                <span className="badge">Google Agenda conectada</span>
              ) : (
                <a className="agenda-connect-link" href="/api/google/oauth/start">
                  Conectar Google Agenda
                </a>
              )}
            </div>
          </div>

          <section className="bottom-split">
            <div className="panel">
              <div className="panel-head">
                <h2>Próximos eventos</h2>
              </div>
              <GoogleSyncButton />
              {days.length === 0 ? (
                <p className="empty-note">Nenhum evento nos próximos 30 dias.</p>
              ) : (
                days.map((day) => (
                  <div className="agenda-day" key={day.key}>
                    <h3 className="agenda-day-heading">{day.heading}</h3>
                    <div className="event-list">
                      {day.events.map((e) => (
                        <EventRow key={e.id} event={e} />
                      ))}
                    </div>
                  </div>
                ))
              )}
            </div>

            <NewEventForm />
          </section>
        </div>
      </main>
    </div>
  );
}
