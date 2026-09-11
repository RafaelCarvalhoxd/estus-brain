"use client";

import { useActionState, useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Event } from "@/lib/agenda";
import {
  createEventAction,
  deleteEventAction,
  syncGoogleAction,
  updateEventAction,
  type CreateEventState,
} from "@/app/agenda/actions";
import { IconPencil, IconTrash } from "./icons";

function IconSync() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 12a8 8 0 0 1 14-5.2M20 12a8 8 0 0 1-14 5.2" />
      <path d="M18 3v4h-4M6 21v-4h4" />
    </svg>
  );
}

function formatTimeRange(startsAt: string, endsAt: string): string {
  const start = new Date(startsAt);
  const end = new Date(endsAt);
  const startLabel = start.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  const endLabel = end.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  return `${startLabel} – ${endLabel}`;
}

function toDateInputValue(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function toTimeInputValue(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function EditEventRow({ event, onDone }: { event: Event; onDone: () => void }) {
  const router = useRouter();
  const [title, setTitle] = useState(event.title);
  const [location, setLocation] = useState(event.location);
  const [date, setDate] = useState(toDateInputValue(event.starts_at));
  const [startTime, setStartTime] = useState(toTimeInputValue(event.starts_at));
  const [endTime, setEndTime] = useState(toTimeInputValue(event.ends_at));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    const result = await updateEventAction(event.id, title, location, date, startTime, endTime);
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onDone();
  }

  return (
    <div className="event-row event-row-editing">
      <div className="event-edit-grid">
        <input className="txn-edit-input" value={title} onChange={(e) => setTitle(e.target.value)} disabled={saving} />
        <input
          className="txn-edit-input"
          value={location}
          onChange={(e) => setLocation(e.target.value)}
          placeholder="Local"
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
          value={startTime}
          onChange={(e) => setStartTime(e.target.value)}
          disabled={saving}
        />
        <input
          className="txn-edit-input"
          type="time"
          value={endTime}
          onChange={(e) => setEndTime(e.target.value)}
          disabled={saving}
        />
      </div>
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

export function EventRow({ event }: { event: Event }) {
  const [isPending, startTransition] = useTransition();
  const [editing, setEditing] = useState(false);

  if (editing) {
    return <EditEventRow event={event} onDone={() => setEditing(false)} />;
  }

  return (
    <div className="event-row">
      <div className="event-body">
        <span className="event-time">{formatTimeRange(event.starts_at, event.ends_at)}</span>
        <div className="event-info">
          <span className="event-title">{event.title}</span>
          {event.location && <span className="event-location">{event.location}</span>}
        </div>
        {event.google_event_id && <span className="badge">Google</span>}
      </div>
      <div className="row-actions">
        <button className="icon-btn" type="button" aria-label="Editar evento" onClick={() => setEditing(true)}>
          <IconPencil />
        </button>
        <button
          type="button"
          className="icon-btn bad"
          aria-label="Excluir evento"
          disabled={isPending}
          onClick={() => startTransition(() => deleteEventAction(event.id))}
        >
          <IconTrash />
        </button>
      </div>
    </div>
  );
}

const initialEventFormState: CreateEventState = { status: "idle" };

export function NewEventForm({
  onSuccess,
  defaultDate,
}: { onSuccess?: () => void; defaultDate?: string } = {}) {
  const [state, formAction, pending] = useActionState(createEventAction, initialEventFormState);

  useEffect(() => {
    if (state.status === "success") onSuccess?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Novo evento</h2>
      </div>
      <form action={formAction} className="form-grid">
        <div className="field">
          <label htmlFor="e-title">Título</label>
          <input id="e-title" name="title" type="text" placeholder="Ex: Reunião com o time" required />
        </div>
        <div className="field">
          <label htmlFor="e-location">Local (opcional)</label>
          <input id="e-location" name="location" type="text" placeholder="Ex: Sala 3, link da chamada" />
        </div>
        <div className="field">
          <label htmlFor="e-date">Data</label>
          <input id="e-date" name="date" type="date" defaultValue={defaultDate} required />
        </div>
        <div className="row2">
          <div className="field">
            <label htmlFor="e-start">Início</label>
            <input id="e-start" name="start_time" type="time" required />
          </div>
          <div className="field">
            <label htmlFor="e-end">Fim</label>
            <input id="e-end" name="end_time" type="time" required />
          </div>
        </div>
        <div className="field">
          <label htmlFor="e-notes">Notas (opcional)</label>
          <input id="e-notes" name="notes" type="text" placeholder="Detalhes adicionais" />
        </div>
        <button className="btn-block" type="submit" disabled={pending}>
          {pending ? "Salvando…" : "Adicionar evento"}
        </button>
        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}

export function GoogleSyncButton() {
  const [isPending, startTransition] = useTransition();
  const [state, setState] = useState<{ status: "idle" | "error" | "success"; message?: string }>({ status: "idle" });

  return (
    <div className="sync-block">
      <button
        type="button"
        className="btn-primary"
        disabled={isPending}
        onClick={() =>
          startTransition(async () => {
            const result = await syncGoogleAction();
            setState(result);
          })
        }
      >
        <IconSync />
        {isPending ? "Sincronizando…" : "Sincronizar agora"}
      </button>
      {state.status === "error" && <p className="form-error">{state.message}</p>}
      {state.status === "success" && <p className="form-success">{state.message}</p>}
    </div>
  );
}
