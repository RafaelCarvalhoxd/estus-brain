"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { createPortal } from "react-dom";
import { useRouter } from "next/navigation";
import { EditorContent, useEditor, type Editor, type JSONContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import Highlight from "@tiptap/extension-highlight";
import Typography from "@tiptap/extension-typography";
import Image from "@tiptap/extension-image";
import { TableKit } from "@tiptap/extension-table";
import CodeBlockLowlight from "@tiptap/extension-code-block-lowlight";
import { CharacterCount, Placeholder } from "@tiptap/extensions";
import { common, createLowlight } from "lowlight";
import type { Note, NoteCategory } from "@/lib/notes";
import { deleteNoteAction } from "@/app/notas/actions";
import { EditorToolbar } from "./EditorToolbar";
import { imageFiles, imageToDataURL } from "./images";
import { SlashCommand, createSlashStore, slashItems, type SlashStore } from "./slash";

const lowlight = createLowlight(common);
const SAVE_DELAY_MS = 900;
const PICK_IMAGE_EVENT = "note-pick-image";

type SaveStatus = "saved" | "dirty" | "saving" | "error";

/** What the list beside the editor needs to stay current while you type. */
export interface NoteMeta {
  id: string;
  title: string;
  body: string;
  pinned: boolean;
  category_id?: string;
  updated_at: string;
}

// Notes from before the editor are plain text: each line becomes a paragraph.
function contentOf(note: Note): JSONContent {
  if (note.content && (note.content as { type?: string }).type === "doc") return note.content as JSONContent;
  return {
    type: "doc",
    content: note.body.split("\n").map((line) =>
      line ? { type: "paragraph", content: [{ type: "text", text: line }] } : { type: "paragraph" },
    ),
  };
}

interface Draft {
  title: string;
  pinned: boolean;
  categoryId: string | null;
}

// The autosave, outside React like the one in Quadros: every edit bumps a
// version, a save sends the note as it is and remembers which version that
// was. Serializing the document (images and all) only happens on a save,
// never per keystroke.
function createAutosave(noteId: string, initial: Draft) {
  let draft = initial;
  let editor: Editor | null = null;
  let onSaved: (meta: NoteMeta) => void = () => {};
  let status: SaveStatus = "saved";
  let error: string | null = null;
  let snapshot: { status: SaveStatus; error: string | null } = { status, error };
  const listeners = new Set<() => void>();
  const report = (next: SaveStatus, err: string | null = null) => {
    if (next === status && err === error) return;
    status = next;
    error = err;
    snapshot = { status, error };
    listeners.forEach((l) => l());
  };

  let version = 0;
  let savedVersion = 0;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let inFlight = false;
  let queued = false;

  async function save(): Promise<boolean> {
    if (timer) clearTimeout(timer);
    timer = null;
    if (inFlight) {
      queued = true;
      return false;
    }
    if (version === savedVersion) return false;

    if (!editor) return false;
    const target = version;
    let body: string;
    let content: JSONContent;
    try {
      body = editor.getText({ blockSeparator: "\n" }).trim();
      content = editor.getJSON();
    } catch {
      return false;
    }

    inFlight = true;
    report("saving");
    try {
      const res = await fetch(`/api/notes/${noteId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: draft.title.trim(),
          body,
          content,
          pinned: draft.pinned,
          category_id: draft.categoryId,
        }),
      });
      const data = (await res.json().catch(() => null)) as { updated_at?: string; error?: string } | null;
      if (!res.ok) throw new Error(data?.error ?? "Não foi possível salvar a nota.");
      savedVersion = target;
      onSaved({
        id: noteId,
        title: draft.title.trim(),
        body,
        pinned: draft.pinned,
        category_id: draft.categoryId ?? undefined,
        updated_at: data?.updated_at ?? new Date().toISOString(),
      });
      report(version === savedVersion ? "saved" : "dirty");
      return true;
    } catch (err) {
      report("error", err instanceof Error ? err.message : "Não foi possível salvar a nota.");
      return false;
    } finally {
      inFlight = false;
      if (queued) {
        queued = false;
        void save();
      }
    }
  }

  function touch() {
    version++;
    if (!inFlight) report("dirty");
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => void save(), SAVE_DELAY_MS);
  }

  return {
    save,
    touch,
    attach(e: Editor) {
      editor = e;
    },
    editor: () => editor,
    setOnSaved(fn: (meta: NoteMeta) => void) {
      onSaved = fn;
    },
    setDraft(patch: Partial<Draft>) {
      draft = { ...draft, ...patch };
      touch();
    },
    /** Nothing written at all — no title, no text, no image or table. */
    isEmpty(): boolean {
      try {
        return !draft.title.trim() && !!editor && editor.isEmpty;
      } catch {
        return false;
      }
    },
    pending: () => inFlight || version !== savedVersion,
    subscribe(l: () => void) {
      listeners.add(l);
      return () => {
        listeners.delete(l);
      };
    },
    getState: () => snapshot,
  };
}

type Autosave = ReturnType<typeof createAutosave>;

export function NoteEditor({
  note,
  categories,
  onSaved,
  onDeleted,
}: {
  note: Note;
  categories: NoteCategory[];
  onSaved: (meta: NoteMeta) => void;
  onDeleted: (id: string) => void;
}) {
  const router = useRouter();
  const [title, setTitle] = useState(note.title);
  const [pinned, setPinned] = useState(note.pinned);
  const [categoryId, setCategoryId] = useState<string | null>(note.category_id ?? null);
  const fileInput = useRef<HTMLInputElement>(null);
  const titleRef = useRef<HTMLTextAreaElement>(null);
  const deletedRef = useRef(false);
  const leaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [autosave] = useState<Autosave>(() =>
    createAutosave(note.id, { title: note.title, pinned: note.pinned, categoryId: note.category_id ?? null }),
  );
  const [slash] = useState<SlashStore>(() => createSlashStore());
  useEffect(() => autosave.setOnSaved(onSaved), [autosave, onSaved]);

  const insertImages = useCallback(async (editor: Editor, files: File[], pos?: number) => {
    for (const file of files) {
      try {
        const src = await imageToDataURL(file);
        const chain = editor.chain().focus();
        if (pos !== undefined) chain.insertContentAt(pos, { type: "image", attrs: { src, alt: file.name } }).run();
        else chain.setImage({ src, alt: file.name }).run();
      } catch {
        // An image the browser can't decode is skipped.
      }
    }
  }, []);

  const [extensions] = useState(() => [
    StarterKit.configure({
      codeBlock: false,
      heading: { levels: [1, 2, 3] },
      link: { openOnClick: false, autolink: true, defaultProtocol: "https" },
    }),
    Placeholder.configure({
      placeholder: ({ node }) => (node.type.name === "heading" ? "Título" : "Escreva, ou digite / para inserir um bloco…"),
    }),
    TaskList,
    TaskItem.configure({ nested: true }),
    Highlight,
    Typography,
    Image.configure({ allowBase64: true }),
    TableKit.configure({ table: { resizable: false } }),
    CodeBlockLowlight.configure({ lowlight }),
    CharacterCount,
    // The menu can't reach the file picker from inside the editor, so it asks
    // for one with an event the component listens for.
    SlashCommand(
      slash,
      slashItems((e) => e.view.dom.dispatchEvent(new CustomEvent(PICK_IMAGE_EVENT, { bubbles: true }))),
    ),
  ]);

  const editor = useEditor({
    extensions,
    content: contentOf(note),
    immediatelyRender: false,
    editorProps: {
      attributes: { class: "nt-doc", spellcheck: "true", "aria-label": "Conteúdo da nota" },
      handlePaste: (view, event) => {
        const files = imageFiles(event.clipboardData?.files);
        const target = autosave.editor();
        if (files.length === 0 || !target) return false;
        event.preventDefault();
        void insertImages(target, files);
        return true;
      },
      handleDrop: (view, event) => {
        const files = imageFiles(event.dataTransfer?.files);
        const target = autosave.editor();
        if (files.length === 0 || !target) return false;
        event.preventDefault();
        const pos = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
        void insertImages(target, files, pos);
        return true;
      },
    },
    onCreate: ({ editor: e }) => autosave.attach(e),
    onUpdate: () => autosave.touch(),
  });

  const { status, error } = useSyncExternalStore(autosave.subscribe, autosave.getState, autosave.getState);
  const words = useSyncExternalStore(
    useCallback((l: () => void) => {
      editor?.on("update", l);
      return () => {
        editor?.off("update", l);
      };
    }, [editor]),
    () => (editor ? (editor.storage.characterCount?.words?.() as number | undefined) ?? 0 : 0),
    () => 0,
  );

  const updateDraft = (patch: Partial<Draft>) => autosave.setDraft(patch);

  useEffect(() => {
    if (!editor) return;
    const dom = editor.view.dom;
    const pick = () => fileInput.current?.click();
    dom.addEventListener(PICK_IMAGE_EVENT, pick);
    return () => dom.removeEventListener(PICK_IMAGE_EVENT, pick);
  }, [editor]);

  // Title grows with its text instead of scrolling.
  useEffect(() => {
    const el = titleRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, [title]);

  // A fresh, empty note starts with the cursor in the title.
  useEffect(() => {
    if (!note.title && !note.body) titleRef.current?.focus();
  }, [note.title, note.body]);

  // ⌘S saves right away; the browser's own "save page" is never what you want here.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        void autosave.save();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [autosave]);

  // Leaving the note saves what's pending. A note left with nothing in it is
  // removed instead — "Nova nota" and walking away shouldn't leave litter.
  // The decision waits a tick: in development React unmounts and remounts
  // every effect once on purpose, and that must not delete a brand-new note.
  useEffect(() => {
    if (leaveTimer.current) clearTimeout(leaveTimer.current);
    const warn = (e: BeforeUnloadEvent) => {
      if (!autosave.pending()) return;
      void autosave.save();
      e.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
      if (deletedRef.current) return;
      const empty = autosave.isEmpty();
      leaveTimer.current = setTimeout(() => {
        if (empty) {
          void deleteNoteAction(note.id).then(() => router.refresh());
          return;
        }
        void autosave.save().then((saved) => saved && router.refresh());
      }, 0);
    };
  }, [autosave, note.id, router]);

  const remove = async () => {
    if (!window.confirm(`Excluir "${title.trim() || "Sem título"}"? Não dá para desfazer.`)) return;
    deletedRef.current = true;
    const result = await deleteNoteAction(note.id);
    if (result.error) {
      deletedRef.current = false;
      window.alert(result.error);
      return;
    }
    onDeleted(note.id);
  };

  const statusLabel =
    status === "saving" ? "Salvando…" : status === "dirty" ? "Alterações pendentes" : status === "error" ? "Erro ao salvar" : "Salvo";

  return (
    <div className="nt-editor">
      <header className="nt-head">
        <select
          className="nt-category"
          aria-label="Caderno"
          value={categoryId ?? ""}
          onChange={(e) => {
            const next = e.target.value || null;
            setCategoryId(next);
            updateDraft({ categoryId: next });
          }}
        >
          <option value="">Geral</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
        <span className={`nt-status is-${status}`} title={error ?? statusLabel}>
          <span className="nt-status-dot" aria-hidden="true" />
          {statusLabel}
        </span>
        <div className="nt-head-actions">
          <button
            type="button"
            className={`icon-btn nt-pin${pinned ? " is-on" : ""}`}
            aria-pressed={pinned}
            title={pinned ? "Desafixar" : "Fixar no topo"}
            aria-label={pinned ? "Desafixar" : "Fixar no topo"}
            onClick={() => {
              setPinned(!pinned);
              updateDraft({ pinned: !pinned });
            }}
          >
            <svg viewBox="0 0 24 24" fill={pinned ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round">
              <path d="M12 2 9 9l-5 1.5L11 18v4l1-1 1 1v-4l7-7.5L15 9Z" />
            </svg>
          </button>
          <button type="button" className="icon-btn bad" title="Excluir nota" aria-label="Excluir nota" onClick={remove}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
              <path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3" />
            </svg>
          </button>
        </div>
      </header>

      {editor && <EditorToolbar editor={editor} onImage={() => fileInput.current?.click()} />}

      <div
        className="nt-scroll"
        // Clicking the empty paper below the text continues writing at the end.
        onMouseDown={(e) => {
          if (editor && (e.target === e.currentTarget || (e.target as HTMLElement).classList.contains("nt-page"))) {
            e.preventDefault();
            editor.commands.focus("end");
          }
        }}
      >
        <div className="nt-page">
          <textarea
            ref={titleRef}
            className="nt-title"
            placeholder="Sem título"
            rows={1}
            maxLength={300}
            value={title}
            aria-label="Título da nota"
            onChange={(e) => {
              setTitle(e.target.value.replace(/\n/g, ""));
              updateDraft({ title: e.target.value.replace(/\n/g, "") });
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" || (e.key === "ArrowDown" && e.currentTarget.selectionStart === title.length)) {
                e.preventDefault();
                editor?.commands.focus("start");
              }
            }}
          />
          <EditorContent editor={editor} />
        </div>
      </div>

      <footer className="nt-foot">
        <span>{words === 1 ? "1 palavra" : `${words} palavras`}</span>
        <span>Digite / para blocos · cole imagens direto</span>
      </footer>

      <input
        ref={fileInput}
        type="file"
        accept="image/*"
        multiple
        hidden
        onChange={(e) => {
          const target = editor;
          const files = imageFiles(e.target.files);
          e.target.value = "";
          if (target && files.length) void insertImages(target, files);
        }}
      />

      <SlashMenu store={slash} />
    </div>
  );
}

function SlashMenu({ store }: { store: SlashStore }) {
  const state = useSyncExternalStore(store.subscribe, store.get, store.get);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listRef.current?.querySelector(".is-selected")?.scrollIntoView({ block: "nearest" });
  }, [state.index]);

  if (!state.open || !state.rect) return null;
  const below = state.rect.bottom + 320 < window.innerHeight;
  const style = below
    ? { left: state.rect.left, top: state.rect.bottom + 6 }
    : { left: state.rect.left, bottom: window.innerHeight - state.rect.top + 6 };

  // Rendered on <body>: the glass panels around the editor use backdrop-filter,
  // which would otherwise make "fixed" mean "fixed to the panel".
  return createPortal(
    <div className="nt-slash" style={style} ref={listRef} role="listbox" aria-label="Inserir bloco">
      {state.items.map((item, i) => (
        <button
          key={item.id}
          type="button"
          role="option"
          aria-selected={i === state.index}
          className={i === state.index ? "is-selected" : ""}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => state.pick(item)}
        >
          <b>{item.label}</b>
          <span>{item.hint}</span>
        </button>
      ))}
    </div>,
    document.body,
  );
}
