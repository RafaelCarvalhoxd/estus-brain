"use client";

import { useActionState, useState, useTransition } from "react";
import type { Event } from "@/lib/agenda";
import { createEventAction, deleteEventAction, initialCreateEventState, syncGoogleAction } from "@/app/agenda/actions";

function IconTrash() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 7h16" />
      <path d="M9 7V4.5A1.5 1.5 0 0 1 10.5 3h3A1.5 1.5 0 0 1 15 4.5V7" />
      <path d="M6 7l1 13.5A1.5 1.5 0 0 0 8.5 22h7a1.5 1.5 0 0 0 1.5-1.5L18 7" />
    </svg>
  );
}

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

export function EventRow({ event }: { event: Event }) {
  const [isPending, startTransition] = useTransition();

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
      <button
        type="button"
        className="event-delete"
        aria-label="Excluir evento"
        disabled={isPending}
        onClick={() => startTransition(() => deleteEventAction(event.id))}
      >
        <IconTrash />
      </button>
    </div>
  );
}

export function NewEventForm() {
  const [state, formAction, pending] = useActionState(createEventAction, initialCreateEventState);

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
          <input id="e-date" name="date" type="date" required />
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
