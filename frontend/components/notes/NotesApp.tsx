"use client";

import { useMemo, useState, useTransition, type CSSProperties } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Note, NoteCategory } from "@/lib/notes";
import {
  createNoteAction,
  createNoteCategoryAction,
  deleteNoteCategoryAction,
  openDailyNoteAction,
  renameNoteCategoryAction,
} from "@/app/notas/actions";
import { IconPencil, IconPlus, IconSearch, IconTrash } from "../icons";
import { NoteEditor, type NoteMeta } from "./NoteEditor";

// Notas as a writing app: notebooks on the left, the notes in the chosen one
// in the middle, the open note on the right. Which notebook and which note
// live in the URL (?c=…&n=…), so the back button and reloads just work.

const PINNED = "fixadas";
const GENERAL = "geral";

function hrefFor(filter: string | null, noteId?: string | null): string {
  const params = new URLSearchParams();
  if (filter) params.set("c", filter);
  if (noteId) params.set("n", noteId);
  const qs = params.toString();
  return qs ? `/notas?${qs}` : "/notas";
}

function excerpt(note: Pick<Note, "title" | "body">): string {
  const lines = note.body.split("\n").map((l) => l.trim());
  return lines.find((l) => l && l !== note.title.trim()) ?? "";
}

function whenLabel(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const sameDay = (a: Date, b: Date) => a.toDateString() === b.toDateString();
  if (sameDay(d, now)) return d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (sameDay(d, yesterday)) return "Ontem";
  return d
    .toLocaleDateString("pt-BR", { day: "numeric", month: "short", year: d.getFullYear() === now.getFullYear() ? undefined : "numeric" })
    .replace(".", "");
}

export function NotesApp({
  notes,
  categories,
  openNote,
  filter,
}: {
  notes: Note[];
  categories: NoteCategory[];
  openNote: Note | null;
  filter: string | null;
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  // What the editor saved since the server last sent the list — kept here so
  // the list follows your typing without refetching everything.
  const [saved, setSaved] = useState<Record<string, NoteMeta>>({});
  const [removed, setRemoved] = useState<Set<string>>(new Set());

  const all = useMemo(() => {
    return notes
      .filter((n) => !removed.has(n.id))
      .map((n) => {
        const s = saved[n.id];
        return s && s.updated_at >= n.updated_at ? { ...n, ...s } : n;
      })
      .sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.updated_at.localeCompare(a.updated_at));
  }, [notes, saved, removed]);

  const counts = useMemo(() => {
    const byCategory: Record<string, number> = {};
    let general = 0;
    let pinned = 0;
    for (const n of all) {
      if (n.pinned) pinned++;
      if (n.category_id) byCategory[n.category_id] = (byCategory[n.category_id] ?? 0) + 1;
      else general++;
    }
    return { byCategory, general, pinned };
  }, [all]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return all.filter((n) => {
      if (filter === PINNED && !n.pinned) return false;
      if (filter === GENERAL && n.category_id) return false;
      if (filter && filter !== PINNED && filter !== GENERAL && n.category_id !== filter) return false;
      return !q || n.title.toLowerCase().includes(q) || n.body.toLowerCase().includes(q);
    });
  }, [all, filter, query]);

  const categoryById = useMemo(() => new Map(categories.map((c) => [c.id, c])), [categories]);
  const filterLabel =
    filter === PINNED ? "Fixadas" : filter === GENERAL ? "Geral" : filter ? (categoryById.get(filter)?.name ?? "Notas") : "Todas as notas";

  const run = (fn: () => Promise<{ error?: string } | void>) => {
    setError(null);
    startTransition(async () => {
      const result = await fn();
      if (result && result.error) setError(result.error);
    });
  };

  const newCategory = () => {
    const name = window.prompt("Nome do caderno", "");
    if (!name?.trim()) return;
    run(async () => {
      const result = await createNoteCategoryAction(name);
      if (result.id) router.push(hrefFor(result.id));
      return result;
    });
  };

  const renameCategory = (c: NoteCategory) => {
    const name = window.prompt("Novo nome do caderno", c.name);
    if (!name?.trim() || name.trim() === c.name) return;
    run(() => renameNoteCategoryAction(c.id, name, c.color));
  };

  const deleteCategory = (c: NoteCategory) => {
    if (!window.confirm(`Excluir o caderno "${c.name}"? As notas dele vão para Geral.`)) return;
    run(async () => {
      const result = await deleteNoteCategoryAction(c.id);
      if (!result.error && filter === c.id) router.push("/notas");
      return result;
    });
  };

  const navItem = (key: string | null, label: string, count: number, color?: string) => (
    <Link
      href={hrefFor(key)}
      className={`nb-item${filter === key ? " is-active" : ""}`}
      style={color ? ({ "--dot": color } as CSSProperties) : undefined}
    >
      {color && <span className="nb-dot" aria-hidden="true" />}
      <span className="nb-name">{label}</span>
      <span className="nb-count">{count}</span>
    </Link>
  );

  return (
    <div className="nx" data-view={openNote ? "editor" : "list"}>
      <aside className="panel nx-side" aria-label="Cadernos">
        <div className="nx-side-actions">
          <button type="button" className="btn-primary" disabled={pending} onClick={() => run(() => createNoteAction(filter))}>
            <IconPlus />
            Nova nota
          </button>
          <button type="button" className="btn-outline" disabled={pending} onClick={() => run(() => openDailyNoteAction())}>
            Nota do dia
          </button>
        </div>

        <nav className="nb-list">
          {navItem(null, "Todas", all.length)}
          {navItem(PINNED, "Fixadas", counts.pinned)}
        </nav>

        <div className="nb-head">
          <span>Cadernos</span>
          <button type="button" className="icon-btn" aria-label="Novo caderno" title="Novo caderno" onClick={newCategory}>
            <IconPlus />
          </button>
        </div>
        <nav className="nb-list">
          {categories.map((c) => (
            <div className="nb-row" key={c.id}>
              {navItem(c.id, c.name, counts.byCategory[c.id] ?? 0, c.color)}
              <div className="nb-row-actions">
                <button type="button" className="icon-btn" aria-label={`Renomear ${c.name}`} onClick={() => renameCategory(c)}>
                  <IconPencil />
                </button>
                <button type="button" className="icon-btn bad" aria-label={`Excluir ${c.name}`} onClick={() => deleteCategory(c)}>
                  <IconTrash />
                </button>
              </div>
            </div>
          ))}
          {navItem(GENERAL, "Geral", counts.general, "var(--ink-mute)")}
        </nav>
      </aside>

      <section className="panel nx-list" aria-label={filterLabel}>
        <div className="nx-list-head">
          <h2>{filterLabel}</h2>
          <select
            className="nx-filter-mobile"
            aria-label="Caderno"
            value={filter ?? ""}
            onChange={(e) => router.push(hrefFor(e.target.value || null))}
          >
            <option value="">Todas</option>
            <option value={PINNED}>Fixadas</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
            <option value={GENERAL}>Geral</option>
          </select>
          <button
            type="button"
            className="icon-btn nx-new-mobile"
            aria-label="Nova nota"
            disabled={pending}
            onClick={() => run(() => createNoteAction(filter))}
          >
            <IconPlus />
          </button>
        </div>
        <label className="nx-search">
          <IconSearch />
          <input type="search" placeholder="Buscar nas notas" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        {error && <p className="form-error nx-error">{error}</p>}

        {visible.length === 0 ? (
          <p className="nx-empty">{query ? "Nenhuma nota com esse texto." : "Nada aqui ainda. Crie uma nota."}</p>
        ) : (
          <ul className="nx-notes">
            {visible.map((n) => {
              const category = n.category_id ? categoryById.get(n.category_id) : undefined;
              return (
                <li key={n.id}>
                  <Link
                    href={hrefFor(filter, n.id)}
                    scroll={false}
                    className={`nx-note${openNote?.id === n.id ? " is-open" : ""}`}
                  >
                    <span className="nx-note-title">
                      {n.pinned && <span className="nx-pin" aria-label="Fixada" />}
                      {n.title.trim() || "Sem título"}
                    </span>
                    <span className="nx-note-excerpt">{excerpt(n) || "Sem texto"}</span>
                    <span className="nx-note-meta">
                      {!filter || filter === PINNED ? (
                        <span className="nx-note-cat" style={{ "--dot": category?.color ?? "var(--ink-mute)" } as CSSProperties}>
                          {category?.name ?? "Geral"}
                        </span>
                      ) : (
                        <span />
                      )}
                      <span>{whenLabel(n.updated_at)}</span>
                    </span>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <section className="panel nx-editor" aria-label="Nota">
        {openNote ? (
          <>
            <Link href={hrefFor(filter)} className="nx-back">
              ‹ {filterLabel}
            </Link>
            <NoteEditor
              key={openNote.id}
              note={openNote}
              categories={categories}
              onSaved={(meta) => setSaved((prev) => ({ ...prev, [meta.id]: meta }))}
              onDeleted={(id) => {
                setRemoved((prev) => new Set(prev).add(id));
                router.push(hrefFor(filter));
              }}
            />
          </>
        ) : (
          <div className="nx-blank">
            <p>Escolha uma nota ao lado, ou comece uma nova.</p>
            <p className="nx-blank-hint">
              Dentro da nota, digite <kbd>/</kbd> para títulos, listas, checklists, código, tabelas e imagens.
            </p>
          </div>
        )}
      </section>
    </div>
  );
}
