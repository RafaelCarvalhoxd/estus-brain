"use client";

import { useActionState, useTransition } from "react";
import type { Reminder } from "@/lib/reminders";
import {
  createReminderAction,
  deleteReminderAction,
  initialCreateReminderState,
  toggleReminderAction,
} from "@/app/lembretes/actions";

function IconTrash() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 7h16" />
      <path d="M9 7V4.5A1.5 1.5 0 0 1 10.5 3h3A1.5 1.5 0 0 1 15 4.5V7" />
      <path d="M6 7l1 13.5A1.5 1.5 0 0 0 8.5 22h7a1.5 1.5 0 0 0 1.5-1.5L18 7" />
    </svg>
  );
}

function formatDueAt(dueAt?: string): string | null {
  if (!dueAt) return null;
  const d = new Date(dueAt);
  const hasTime = d.getHours() !== 0 || d.getMinutes() !== 0;
  const datePart = d.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
  if (!hasTime) return datePart;
  const timePart = d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  return `${datePart} às ${timePart}`;
}

function ReminderRow({ reminder, bucket }: { reminder: Reminder; bucket: string }) {
  const [isPending, startTransition] = useTransition();
  const due = formatDueAt(reminder.due_at);

  return (
    <div className="reminder-row">
      <input
        type="checkbox"
        className="reminder-check"
        checked={reminder.done}
        disabled={isPending}
        onChange={(e) => startTransition(() => toggleReminderAction(reminder.id, e.target.checked))}
        aria-label={reminder.done ? "Marcar como não concluído" : "Marcar como concluído"}
      />
      <div className="reminder-body">
        <span className={`reminder-title ${reminder.done ? "reminder-title-done" : ""}`}>{reminder.title}</span>
        {due && <span className={`badge ${bucket === "atrasado" ? "badge-bad" : ""}`}>{due}</span>}
      </div>
      <button
        type="button"
        className="reminder-delete"
        aria-label="Excluir lembrete"
        disabled={isPending}
        onClick={() => startTransition(() => deleteReminderAction(reminder.id))}
      >
        <IconTrash />
      </button>
    </div>
  );
}

export function ReminderSection({ title, reminders, bucket }: { title: string; reminders: Reminder[]; bucket: string }) {
  if (reminders.length === 0) return null;
  return (
    <div className="reminder-section">
      <h2 className="reminder-section-title">{title}</h2>
      <div className="reminder-list">
        {reminders.map((r) => (
          <ReminderRow key={r.id} reminder={r} bucket={bucket} />
        ))}
      </div>
    </div>
  );
}

export function NewReminderForm() {
  const [state, formAction, pending] = useActionState(createReminderAction, initialCreateReminderState);

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Novo lembrete</h2>
      </div>
      <form action={formAction} className="reminder-form">
        <div className="field">
          <label htmlFor="r-title">Título</label>
          <input id="r-title" name="title" type="text" placeholder="Ex: Renovar seguro do carro" required />
        </div>
        <div className="row2">
          <div className="field">
            <label htmlFor="r-date">Data (opcional)</label>
            <input id="r-date" name="due_date" type="date" />
          </div>
          <div className="field">
            <label htmlFor="r-time">Horário (opcional)</label>
            <input id="r-time" name="due_time" type="time" />
          </div>
        </div>
        <button className="btn-block" type="submit" disabled={pending}>
          {pending ? "Salvando…" : "Adicionar lembrete"}
        </button>
        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}
