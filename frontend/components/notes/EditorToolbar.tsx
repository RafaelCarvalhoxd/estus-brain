"use client";

import type { ReactNode } from "react";
import { useEditorState, type Editor } from "@tiptap/react";

// A fixed toolbar for what the "/" menu and markdown shortcuts also do, so
// nothing depends on remembering a shortcut. Table controls appear only
// while the cursor is in a table.

const svg = (children: ReactNode) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    {children}
  </svg>
);

const ICON = {
  bullet: svg(
    <>
      <line x1="9" y1="6" x2="20" y2="6" />
      <line x1="9" y1="12" x2="20" y2="12" />
      <line x1="9" y1="18" x2="20" y2="18" />
      <circle cx="4.5" cy="6" r="1" fill="currentColor" />
      <circle cx="4.5" cy="12" r="1" fill="currentColor" />
      <circle cx="4.5" cy="18" r="1" fill="currentColor" />
    </>,
  ),
  ordered: svg(
    <>
      <line x1="10" y1="6" x2="20" y2="6" />
      <line x1="10" y1="12" x2="20" y2="12" />
      <line x1="10" y1="18" x2="20" y2="18" />
      <path d="M4 5h1.5v4M4 9h3" />
      <path d="M4 14.5c.4-.6 1-1 1.7-1 .8 0 1.3.5 1.3 1.1 0 1.2-3 1.9-3 3.4h3" />
    </>,
  ),
  task: svg(
    <>
      <rect x="3.5" y="4" width="6" height="6" rx="1.5" />
      <path d="m5 7 1.2 1.2L8.4 6" />
      <rect x="3.5" y="14" width="6" height="6" rx="1.5" />
      <line x1="13" y1="7" x2="20" y2="7" />
      <line x1="13" y1="17" x2="20" y2="17" />
    </>,
  ),
  quote: svg(
    <>
      <path d="M5 7h5v5H7c0 2 1 3 3 3.5" />
      <path d="M14 7h5v5h-3c0 2 1 3 3 3.5" />
    </>,
  ),
  code: svg(
    <>
      <path d="m8 8-4 4 4 4" />
      <path d="m16 8 4 4-4 4" />
    </>,
  ),
  codeBlock: svg(
    <>
      <rect x="3" y="4" width="18" height="16" rx="2.5" />
      <path d="m9.5 10-2 2 2 2M14.5 10l2 2-2 2" />
    </>,
  ),
  table: svg(
    <>
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <line x1="3" y1="10" x2="21" y2="10" />
      <line x1="10" y1="10" x2="10" y2="20" />
    </>,
  ),
  image: svg(
    <>
      <rect x="3" y="4" width="18" height="16" rx="2.5" />
      <circle cx="9" cy="10" r="1.8" />
      <path d="m21 16-5-5-9 9" />
    </>,
  ),
  hr: svg(<line x1="4" y1="12" x2="20" y2="12" />),
  highlight: svg(
    <>
      <path d="m9 15 7.5-7.5a2.1 2.1 0 0 0-3-3L6 12l-1 4 4-1Z" />
      <line x1="4" y1="20" x2="20" y2="20" />
    </>,
  ),
  link: svg(
    <>
      <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" />
      <path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
    </>,
  ),
  undo: svg(
    <>
      <path d="M9 14 4 9l5-5" />
      <path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11" />
    </>,
  ),
  redo: svg(
    <>
      <path d="m15 14 5-5-5-5" />
      <path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13" />
    </>,
  ),
};

function Btn({
  label,
  active = false,
  disabled = false,
  onClick,
  children,
}: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={`nt-btn${active ? " is-active" : ""}`}
      title={label}
      aria-label={label}
      aria-pressed={active}
      disabled={disabled}
      // Keep the selection in the editor while clicking.
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function EditorToolbar({ editor, onImage }: { editor: Editor; onImage: () => void }) {
  const s = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      block: e.isActive("heading", { level: 1 })
        ? "h1"
        : e.isActive("heading", { level: 2 })
          ? "h2"
          : e.isActive("heading", { level: 3 })
            ? "h3"
            : "p",
      bold: e.isActive("bold"),
      italic: e.isActive("italic"),
      underline: e.isActive("underline"),
      strike: e.isActive("strike"),
      code: e.isActive("code"),
      highlight: e.isActive("highlight"),
      link: e.isActive("link"),
      bullet: e.isActive("bulletList"),
      ordered: e.isActive("orderedList"),
      task: e.isActive("taskList"),
      quote: e.isActive("blockquote"),
      codeBlock: e.isActive("codeBlock"),
      table: e.isActive("table"),
      canUndo: e.can().undo(),
      canRedo: e.can().redo(),
    }),
  });

  const chain = () => editor.chain().focus();

  const setLink = () => {
    const previous = (editor.getAttributes("link").href as string | undefined) ?? "";
    const url = window.prompt("Endereço do link", previous || "https://");
    if (url === null) return;
    if (url.trim() === "" || url.trim() === "https://") {
      chain().extendMarkRange("link").unsetLink().run();
      return;
    }
    chain().extendMarkRange("link").setLink({ href: url.trim() }).run();
  };

  return (
    <div className="nt-toolbar" role="toolbar" aria-label="Formatação">
      <select
        className="nt-block"
        aria-label="Tipo de texto"
        value={s.block}
        onChange={(e) => {
          const v = e.target.value;
          if (v === "p") chain().setParagraph().run();
          else chain().setHeading({ level: Number(v.slice(1)) as 1 | 2 | 3 }).run();
        }}
      >
        <option value="p">Texto</option>
        <option value="h1">Título grande</option>
        <option value="h2">Título médio</option>
        <option value="h3">Título pequeno</option>
      </select>

      <span className="nt-sep" />
      <Btn label="Negrito (⌘B)" active={s.bold} onClick={() => chain().toggleBold().run()}>
        <b>B</b>
      </Btn>
      <Btn label="Itálico (⌘I)" active={s.italic} onClick={() => chain().toggleItalic().run()}>
        <i>I</i>
      </Btn>
      <Btn label="Sublinhado (⌘U)" active={s.underline} onClick={() => chain().toggleUnderline().run()}>
        <u>U</u>
      </Btn>
      <Btn label="Riscado" active={s.strike} onClick={() => chain().toggleStrike().run()}>
        <s>S</s>
      </Btn>
      <Btn label="Marca-texto" active={s.highlight} onClick={() => chain().toggleHighlight().run()}>
        {ICON.highlight}
      </Btn>
      <Btn label="Código no texto" active={s.code} onClick={() => chain().toggleCode().run()}>
        {ICON.code}
      </Btn>
      <Btn label="Link" active={s.link} onClick={setLink}>
        {ICON.link}
      </Btn>

      <span className="nt-sep" />
      <Btn label="Lista" active={s.bullet} onClick={() => chain().toggleBulletList().run()}>
        {ICON.bullet}
      </Btn>
      <Btn label="Lista numerada" active={s.ordered} onClick={() => chain().toggleOrderedList().run()}>
        {ICON.ordered}
      </Btn>
      <Btn label="Checklist" active={s.task} onClick={() => chain().toggleTaskList().run()}>
        {ICON.task}
      </Btn>
      <Btn label="Citação" active={s.quote} onClick={() => chain().toggleBlockquote().run()}>
        {ICON.quote}
      </Btn>
      <Btn label="Bloco de código" active={s.codeBlock} onClick={() => chain().toggleCodeBlock().run()}>
        {ICON.codeBlock}
      </Btn>
      <Btn
        label="Tabela"
        active={s.table}
        onClick={() => chain().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()}
      >
        {ICON.table}
      </Btn>
      <Btn label="Imagem" onClick={onImage}>
        {ICON.image}
      </Btn>
      <Btn label="Divisor" onClick={() => chain().setHorizontalRule().run()}>
        {ICON.hr}
      </Btn>

      <span className="nt-sep" />
      <Btn label="Desfazer (⌘Z)" disabled={!s.canUndo} onClick={() => chain().undo().run()}>
        {ICON.undo}
      </Btn>
      <Btn label="Refazer (⇧⌘Z)" disabled={!s.canRedo} onClick={() => chain().redo().run()}>
        {ICON.redo}
      </Btn>

      {s.table && (
        <div className="nt-table-tools" role="group" aria-label="Tabela">
          <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => chain().addRowAfter().run()}>
            + linha
          </button>
          <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => chain().addColumnAfter().run()}>
            + coluna
          </button>
          <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => chain().deleteRow().run()}>
            − linha
          </button>
          <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => chain().deleteColumn().run()}>
            − coluna
          </button>
          <button type="button" className="bad" onMouseDown={(e) => e.preventDefault()} onClick={() => chain().deleteTable().run()}>
            Excluir tabela
          </button>
        </div>
      )}
    </div>
  );
}
