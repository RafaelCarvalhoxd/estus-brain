"use client";

import { useActionState, useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Reminder } from "@/lib/reminders";
import {
  createReminderAction,
  deleteReminderAction,
  toggleReminderAction,
  updateReminderAction,
  type CreateReminderState,
} from "@/app/lembretes/actions";
import { IconPencil, IconTrash } from "./icons";
import { RepeatPicker, formatRepeat, repeatValueOf, toRepeat } from "./RepeatPicker";

function formatDueAt(dueAt?: string): string | null {
  if (!dueAt) return null;
  const d = new Date(dueAt);
  const hasTime = d.getHours() !== 0 || d.getMinutes() !== 0;
  const datePart = d.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
  if (!hasTime) return datePart;
  const timePart = d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  return `${datePart} às ${timePart}`;
}

function toDateInputValue(dueAt?: string): string {
  if (!dueAt) return "";
  const d = new Date(dueAt);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function toTimeInputValue(dueAt?: string): string {
  if (!dueAt) return "";
  const d = new Date(dueAt);
  if (d.getHours() === 0 && d.getMinutes() === 0) return "";
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function EditReminderRow({ reminder, onDone }: { reminder: Reminder; onDone: () => void }) {
  const router = useRouter();
  const [title, setTitle] = useState(reminder.title);
  const [date, setDate] = useState(toDateInputValue(reminder.due_at));
  const [time, setTime] = useState(toTimeInputValue(reminder.due_at));
  const [repeat, setRepeat] = useState(repeatValueOf(reminder));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    const result = await updateReminderAction(reminder.id, title, date, time, toRepeat(repeat));
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onDone();
  }

  return (
    <div className="reminder-row reminder-row-editing">
      <div className="reminder-edit-grid">
        <input
          className="txn-edit-input"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          disabled={saving}
        />
        <input
          className="txn-edit-input"
          type="date"
          value={date}
          onChange={(e) => setDate(e.target.value)}
          disabled={saving}
        />
        <input
          className="txn-edit-input"
          type="time"
          value={time}
          onChange={(e) => setTime(e.target.value)}
          disabled={saving}
        />
      </div>
      <RepeatPicker value={repeat} onChange={setRepeat} disabled={saving} />
      {error && <p className="form-error">{error}</p>}
      <div className="row-actions">
        <button className="btn-text" type="button" onClick={onDone} disabled={saving}>
          Cancelar
        </button>
        <button className="btn-text" type="button" onClick={save} disabled={saving || !title.trim()}>
          {saving ? "Salvando…" : "Salvar"}
        </button>
      </div>
    </div>
  );
}

function ReminderRow({ reminder, bucket }: { reminder: Reminder; bucket: string }) {
  const [isPending, startTransition] = useTransition();
  const [editing, setEditing] = useState(false);
  const due = formatDueAt(reminder.due_at);
  const repeat = formatRepeat(reminder);

  if (editing) {
    return <EditReminderRow reminder={reminder} onDone={() => setEditing(false)} />;
  }

  return (
    <div className="reminder-row">
      <input
        type="checkbox"
        className="reminder-check"
        checked={reminder.done}
        disabled={isPending}
        onChange={(e) => startTransition(() => toggleReminderAction(reminder.id, e.target.checked))}
        aria-label={
          reminder.done ? "Marcar como não concluído" : repeat ? "Concluir e passar para a próxima vez" : "Marcar como concluído"
        }
      />
      <div className="reminder-body">
        <span className={`reminder-title ${reminder.done ? "reminder-title-done" : ""}`}>{reminder.title}</span>
        {due && <span className={`badge ${bucket === "atrasado" ? "badge-bad" : ""}`}>{due}</span>}
        {repeat && <span className="badge">Repete: {repeat}</span>}
      </div>
      <div className="row-actions">
        <button className="icon-btn" type="button" aria-label="Editar lembrete" onClick={() => setEditing(true)}>
          <IconPencil />
        </button>
        <button
          type="button"
          className="icon-btn bad"
          aria-label="Excluir lembrete"
          disabled={isPending}
          onClick={() => startTransition(() => deleteReminderAction(reminder.id))}
        >
          <IconTrash />
        </button>
      </div>
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

const initialReminderFormState: CreateReminderState = { status: "idle" };

export function NewReminderForm({ onSuccess }: { onSuccess?: () => void } = {}) {
  const [state, formAction, pending] = useActionState(createReminderAction, initialReminderFormState);
  const [repeat, setRepeat] = useState(repeatValueOf());

  useEffect(() => {
    if (state.status === "success") onSuccess?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);

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
        <div className="field">
          <label>Repetir (opcional)</label>
          <RepeatPicker value={repeat} onChange={setRepeat} named />
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
