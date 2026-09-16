import { listReminders, type Reminder } from "@/lib/reminders";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { ReminderSection } from "@/components/ReminderList";
import { NewReminderModal } from "@/components/NewReminderModal";
import "../ui.css";
import "./reminders.css";

function bucketOf(r: Reminder, now: Date): "atrasado" | "hoje" | "proximo" | "concluido" {
  if (r.done) return "concluido";
  if (!r.due_at) return "proximo";
  const due = new Date(r.due_at);
  if (due < now) return "atrasado";
  const sameDay =
    due.getFullYear() === now.getFullYear() && due.getMonth() === now.getMonth() && due.getDate() === now.getDate();
  return sameDay ? "hoje" : "proximo";
}

export default async function LembretesPage() {
  const reminders = await listReminders();
  const now = new Date();

  const groups: Record<string, Reminder[]> = { atrasado: [], hoje: [], proximo: [], concluido: [] };
  for (const r of reminders) {
    groups[bucketOf(r, now)].push(r);
  }

  return (
    <div className="shell">
      <ModuleTopBar module="lembretes" />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Lembretes</h1>
            <NewReminderModal />
          </div>

          <div className="panel">
            <div className="panel-head">
              <h2>Seus lembretes</h2>
            </div>
            {reminders.length === 0 ? (
              <p className="empty-note">Nenhum lembrete por aqui ainda.</p>
            ) : (
              <>
                <ReminderSection title="Atrasados" reminders={groups.atrasado} bucket="atrasado" />
                <ReminderSection title="Hoje" reminders={groups.hoje} bucket="hoje" />
                <ReminderSection title="Próximos" reminders={groups.proximo} bucket="proximo" />
                <ReminderSection title="Concluídos" reminders={groups.concluido} bucket="concluido" />
              </>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
