"use client";

import { useActionState, useEffect, useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Note, NoteCategory } from "@/lib/notes";
import {
  createNoteAction,
  deleteNoteAction,
  updateNoteAction,
  createNoteCategoryAction,
  type NoteFormState,
} from "@/app/notas/actions";
import { Modal } from "./Modal";
import { IconSearch } from "./icons";

const GENERAL_LABEL = "Geral";
const NEW_CATEGORY_VALUE = "__new__";
const NEW_CATEGORY_COLORS = ["#2a78d6", "#eb6834", "#1baf7a", "#c98d00", "#e2588e", "#4a3aa7"];

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

function categoryName(categories: NoteCategory[], categoryId?: string): string {
  if (!categoryId) return GENERAL_LABEL;
  return categories.find((c) => c.id === categoryId)?.name ?? GENERAL_LABEL;
}

function categoryColor(categories: NoteCategory[], categoryId?: string): string {
  if (!categoryId) return "var(--ink-mute)";
  return categories.find((c) => c.id === categoryId)?.color ?? "var(--ink-mute)";
}

function groupNotes(notes: Note[], categories: NoteCategory[]) {
  const pinned = notes.filter((n) => n.pinned);
  const rest = notes.filter((n) => !n.pinned);

  const byCategory = new Map<string, Note[]>();
  for (const n of rest) {
    const key = categoryName(categories, n.category_id);
    if (!byCategory.has(key)) byCategory.set(key, []);
    byCategory.get(key)!.push(n);
  }
  const groups = Array.from(byCategory.entries()).sort(([a], [b]) => {
    if (a === GENERAL_LABEL) return 1;
    if (b === GENERAL_LABEL) return -1;
    return a.localeCompare(b, "pt-BR");
  });

  return { pinned, groups };
}

function NoteRow({ note, categories, onOpen }: { note: Note; categories: NoteCategory[]; onOpen: () => void }) {
  return (
    <button type="button" className="notes-list-item" onClick={onOpen}>
      <div className="notes-list-item-head">
        {note.pinned && <IconPin filled />}
        <span className="notes-list-title">{note.title || "Sem título"}</span>
        <span className="dot" style={{ background: categoryColor(categories, note.category_id) }} />
      </div>
      <p className="notes-list-preview">{firstLine(note.body) || "Sem conteúdo"}</p>
      <span className="notes-list-date">{formatUpdatedAt(note.updated_at)}</span>
    </button>
  );
}

export function NotesBoard({ notes, categories }: { notes: Note[]; categories: NoteCategory[] }) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [query, setQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [openNoteId, setOpenNoteId] = useState<string | null>(null);
  const [creatingNew, setCreatingNew] = useState(false);

  const visibleNotes = useMemo(() => {
    const q = query.trim().toLowerCase();
    return notes.filter((n) => {
      const matchesQuery = !q || n.title.toLowerCase().includes(q) || n.body.toLowerCase().includes(q);
      const matchesCategory =
        !categoryFilter ||
        (categoryFilter === GENERAL_LABEL ? !n.category_id : n.category_id === categoryFilter);
      return matchesQuery && matchesCategory;
    });
  }, [notes, query, categoryFilter]);

  const { pinned, groups } = useMemo(() => groupNotes(visibleNotes, categories), [visibleNotes, categories]);
  const openNote = openNoteId ? notes.find((n) => n.id === openNoteId) ?? null : null;

  function handleDelete(id: string) {
    startTransition(async () => {
      await deleteNoteAction(id);
      router.refresh();
      setOpenNoteId(null);
    });
  }

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Suas notas</h2>
        <button type="button" className="btn-primary notes-new-btn" onClick={() => setCreatingNew(true)}>
          <IconPlus />
          Nova nota
        </button>
      </div>

      {notes.length > 0 && (
        <div className="list-filters">
          <div className="filter-search">
            <IconSearch />
            <input type="text" placeholder="Buscar notas" value={query} onChange={(e) => setQuery(e.target.value)} />
          </div>
          <select value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)}>
            <option value="">Todas as categorias</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
            <option value={GENERAL_LABEL}>Geral</option>
          </select>
        </div>
      )}

      {notes.length === 0 ? (
        <p className="empty-note">Nenhuma nota ainda. Crie a primeira.</p>
      ) : visibleNotes.length === 0 ? (
        <p className="empty-note">Nenhuma nota bate com esse filtro.</p>
      ) : (
        <div className="notes-groups">
          {pinned.length > 0 && (
            <div className="notes-group">
              <h3 className="notes-group-title">Fixadas</h3>
              <ul className="notes-list">
                {pinned.map((note) => (
                  <li key={note.id}>
                    <NoteRow note={note} categories={categories} onOpen={() => setOpenNoteId(note.id)} />
                  </li>
                ))}
              </ul>
            </div>
          )}
          {groups.map(([name, groupNotesList]) => (
            <div className="notes-group" key={name}>
              <h3 className="notes-group-title">{name}</h3>
              <ul className="notes-list">
                {groupNotesList.map((note) => (
                  <li key={note.id}>
                    <NoteRow note={note} categories={categories} onOpen={() => setOpenNoteId(note.id)} />
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}

      <Modal open={creatingNew} onClose={() => setCreatingNew(false)}>
        <div className="panel">
          <div className="panel-head">
            <h2>Nova nota</h2>
          </div>
          <NoteEditor categories={categories} onDone={() => setCreatingNew(false)} />
        </div>
      </Modal>

      <Modal open={openNote !== null} onClose={() => setOpenNoteId(null)}>
        {openNote && (
          <div className="panel">
            <div className="panel-head">
              <h2>Editar nota</h2>
            </div>
            <NoteEditor
              note={openNote}
              categories={categories}
              onDone={() => setOpenNoteId(null)}
              onDelete={() => handleDelete(openNote.id)}
              deleting={isPending}
            />
          </div>
        )}
      </Modal>
    </div>
  );
}

function CategorySelect({
  categories,
  value,
  onChange,
}: {
  categories: NoteCategory[];
  value: string;
  onChange: (id: string) => void;
}) {
  const router = useRouter();
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleCreate() {
    if (!newName.trim()) return;
    setSaving(true);
    setError(null);
    const color = NEW_CATEGORY_COLORS[categories.length % NEW_CATEGORY_COLORS.length];
    const result = await createNoteCategoryAction(newName, color);
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onChange(result.id!);
    setCreating(false);
    setNewName("");
  }

  if (creating) {
    return (
      <div className="category-inline-create">
        <input
          type="text"
          placeholder="Nome da categoria"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          disabled={saving}
          autoFocus
        />
        <button className="btn-text" type="button" onClick={() => setCreating(false)} disabled={saving}>
          Cancelar
        </button>
        <button className="btn-text" type="button" onClick={handleCreate} disabled={saving || !newName.trim()}>
          {saving ? "Criando…" : "Criar"}
        </button>
        {error && <p className="form-error">{error}</p>}
      </div>
    );
  }

  return (
    <select
      value={value}
      onChange={(e) => {
        if (e.target.value === NEW_CATEGORY_VALUE) {
          setCreating(true);
          return;
        }
        onChange(e.target.value);
      }}
    >
      <option value="">Geral</option>
      {categories.map((c) => (
        <option key={c.id} value={c.id}>
          {c.name}
        </option>
      ))}
      <option value={NEW_CATEGORY_VALUE}>+ Nova categoria…</option>
    </select>
  );
}

function NoteEditor({
  note,
  categories,
  onDone,
  onDelete,
  deleting,
}: {
  note?: Note;
  categories: NoteCategory[];
  onDone?: () => void;
  onDelete?: () => void;
  deleting?: boolean;
}) {
  const action = note ? updateNoteAction : createNoteAction;
  const [state, formAction, isPending] = useActionState<NoteFormState, FormData>(action, { status: "idle" });
  const [pinned, setPinned] = useState(note?.pinned ?? false);
  const [categoryId, setCategoryId] = useState(note?.category_id ?? "");

  useEffect(() => {
    if (state.status === "success") onDone?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);

  return (
    <form
      action={(formData) => {
        formData.set("pinned", pinned ? "1" : "0");
        formData.set("category_id", categoryId);
        formAction(formData);
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
        <label>Categoria</label>
        <CategorySelect categories={categories} value={categoryId} onChange={setCategoryId} />
      </div>

      <div className="field">
        <textarea
          name="body"
          placeholder="Escreva sua nota..."
          defaultValue={note?.body ?? ""}
          className="notes-body-input"
          rows={10}
        />
      </div>

      {state.status === "error" && <p className="form-error">{state.message}</p>}

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
