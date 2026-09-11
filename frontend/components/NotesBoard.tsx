"use client";

import { useActionState, useMemo, useState, useTransition } from "react";
import type { Note } from "@/lib/notes";
import { createNoteAction, deleteNoteAction, updateNoteAction, type NoteFormState } from "@/app/notas/actions";

function IconPin({ filled }: { filled: boolean }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 2 9 9l-5 1.5L11 18v4l1-1 1 1v-4l7-7.5L15 9Z" />
    </svg>
  );
}

function IconPlus() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="12" y1="5" x2="12" y2="19" />
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

function firstLine(body: string): string {
  const line = body.split("\n").find((l) => l.trim().length > 0);
  return line ?? "";
}

function formatUpdatedAt(iso: string): string {
  const date = new Date(iso);
  return date.toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" });
}

const emptyFormState: NoteFormState = { status: "idle" };

export function NotesBoard({ notes }: { notes: Note[] }) {
  const [selectedId, setSelectedId] = useState<string | null>(notes[0]?.id ?? null);
  const [creatingNew, setCreatingNew] = useState(notes.length === 0);
  const [isPending, startTransition] = useTransition();

  const selected = useMemo(
    () => (creatingNew ? null : notes.find((n) => n.id === selectedId) ?? null),
    [notes, selectedId, creatingNew],
  );

  function openNote(id: string) {
    setCreatingNew(false);
    setSelectedId(id);
  }

  function openNew() {
    setCreatingNew(true);
    setSelectedId(null);
  }

  function handleDelete(id: string) {
    startTransition(async () => {
      await deleteNoteAction(id);
      setSelectedId(null);
      setCreatingNew(false);
    });
  }

  return (
    <div className="notes-board">
      <section className="panel notes-list-panel">
        <div className="panel-head">
          <h2>Suas notas</h2>
          <button type="button" className="btn-primary notes-new-btn" onClick={openNew}>
            <IconPlus />
            Nova nota
          </button>
        </div>

        {notes.length === 0 ? (
          <p className="empty-note">Nenhuma nota ainda. Crie a primeira.</p>
        ) : (
          <ul className="notes-list">
            {notes.map((note) => (
              <li key={note.id}>
                <button
                  type="button"
                  className={`notes-list-item${note.id === selectedId && !creatingNew ? " active" : ""}`}
                  onClick={() => openNote(note.id)}
                >
                  <div className="notes-list-item-head">
                    {note.pinned && <IconPin filled />}
                    <span className="notes-list-title">{note.title || "Sem título"}</span>
                  </div>
                  <p className="notes-list-preview">{firstLine(note.body) || "Sem conteúdo"}</p>
                  <span className="notes-list-date">{formatUpdatedAt(note.updated_at)}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="panel notes-editor-panel">
        {creatingNew ? (
          <NoteEditor key="new" onDone={() => setCreatingNew(false)} />
        ) : selected ? (
          <NoteEditor key={selected.id} note={selected} onDelete={() => handleDelete(selected.id)} deleting={isPending} />
        ) : (
          <p className="empty-note">Selecione uma nota ou crie uma nova.</p>
        )}
      </section>
    </div>
  );
}

function NoteEditor({
  note,
  onDone,
  onDelete,
  deleting,
}: {
  note?: Note;
  onDone?: () => void;
  onDelete?: () => void;
  deleting?: boolean;
}) {
  const action = note ? updateNoteAction : createNoteAction;
  const [state, formAction, isPending] = useActionState<NoteFormState, FormData>(action, emptyFormState);
  const [pinned, setPinned] = useState(note?.pinned ?? false);

  return (
    <form
      action={(formData) => {
        formData.set("pinned", pinned ? "1" : "0");
        formAction(formData);
        if (!note) onDone?.();
      }}
      className="form-grid"
    >
      {note && <input type="hidden" name="id" value={note.id} />}

      <div className="notes-editor-top">
        <input
          type="text"
          name="title"
          placeholder="Título"
          defaultValue={note?.title ?? ""}
          className="notes-title-input"
        />
        <button
          type="button"
          className={`notes-pin-toggle${pinned ? " active" : ""}`}
          onClick={() => setPinned((p) => !p)}
          title={pinned ? "Desafixar" : "Fixar"}
        >
          <IconPin filled={pinned} />
        </button>
      </div>

      <div className="field">
        <textarea
          name="body"
          placeholder="Escreva sua nota..."
          defaultValue={note?.body ?? ""}
          className="notes-body-input"
          rows={14}
        />
      </div>

      {state.status === "error" && <p className="form-error">{state.message}</p>}
      {state.status === "success" && <p className="form-success">{state.message}</p>}

      <div className="notes-editor-actions">
        <button type="submit" className="btn-block" disabled={isPending}>
          {isPending ? "Salvando..." : "Salvar"}
        </button>
        {note && (
          <button type="button" className="btn-outline bad" onClick={onDelete} disabled={deleting}>
            Excluir
          </button>
        )}
      </div>
    </form>
  );
}
