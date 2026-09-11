import { googleStatus, listEvents, type Event } from "@/lib/agenda";
import { buildMonthGrid, currentYearMonth } from "@/lib/month";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { GoogleSyncButton } from "@/components/EventForm";
import { NewEventModal } from "@/components/NewEventModal";
import { CalendarGrid } from "@/components/CalendarGrid";
import "../ui.css";
import "./agenda.css";

function dayKey(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function groupByDay(events: Event[]): Record<string, Event[]> {
  const groups: Record<string, Event[]> = {};
  for (const e of events) {
    const key = dayKey(e.starts_at);
    (groups[key] ??= []).push(e);
  }
  for (const key of Object.keys(groups)) {
    groups[key].sort((a, b) => (a.starts_at < b.starts_at ? -1 : a.starts_at > b.starts_at ? 1 : 0));
  }
  return groups;
}

export default async function AgendaPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();
  const days = buildMonthGrid(month);

  const from = new Date(`${days[0].iso}T00:00:00`);
  const to = new Date(`${days[days.length - 1].iso}T23:59:59`);

  const [events, google] = await Promise.all([listEvents(from, to), googleStatus()]);
  const eventsByDay = groupByDay(events);

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <TopBar
            month={month}
            basePath="/agenda"
            action={
              <div className="agenda-google-status">
                {google.connected ? (
                  <span className="badge">Google Agenda conectada</span>
                ) : (
                  <a className="agenda-connect-link" href="/api/google/oauth/start">
                    Conectar Google Agenda
                  </a>
                )}
                <NewEventModal />
              </div>
            }
          />

          <div className="panel cal-panel">
            <GoogleSyncButton />
            <CalendarGrid days={days} eventsByDay={eventsByDay} />
          </div>
        </div>
      </main>
    </div>
  );
}
