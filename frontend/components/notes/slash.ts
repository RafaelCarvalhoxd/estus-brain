import { Extension, type Editor, type Range } from "@tiptap/core";
import Suggestion, { type SuggestionProps } from "@tiptap/suggestion";

// The "/" menu: type a slash anywhere to insert a block. Its state lives in a
// small store the React menu reads, so the editor never re-renders for it.

export interface SlashItem {
  id: string;
  label: string;
  hint: string;
  keywords: string;
  run: (editor: Editor, range: Range) => void;
}

export interface SlashState {
  open: boolean;
  items: SlashItem[];
  index: number;
  rect: DOMRect | null;
  pick: (item: SlashItem) => void;
}

const CLOSED: SlashState = { open: false, items: [], index: 0, rect: null, pick: () => {} };

export function createSlashStore() {
  let state = CLOSED;
  const listeners = new Set<() => void>();
  const set = (next: SlashState) => {
    state = next;
    listeners.forEach((l) => l());
  };
  return {
    subscribe(l: () => void) {
      listeners.add(l);
      return () => {
        listeners.delete(l);
      };
    },
    get: () => state,
    set,
    close: () => set(CLOSED),
  };
}

export type SlashStore = ReturnType<typeof createSlashStore>;

function normalize(s: string): string {
  return s.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
}

export function slashItems(onImage: (editor: Editor) => void): SlashItem[] {
  const block = (id: string, label: string, hint: string, keywords: string, apply: (e: Editor) => void): SlashItem => ({
    id,
    label,
    hint,
    keywords,
    run: (editor, range) => {
      editor.chain().focus().deleteRange(range).run();
      apply(editor);
    },
  });
  return [
    block("p", "Texto", "Parágrafo comum", "texto paragrafo p", (e) => e.chain().focus().setParagraph().run()),
    block("h1", "Título grande", "Seção principal", "titulo h1 heading", (e) => e.chain().focus().setHeading({ level: 1 }).run()),
    block("h2", "Título médio", "Subseção", "titulo h2 heading subtitulo", (e) => e.chain().focus().setHeading({ level: 2 }).run()),
    block("h3", "Título pequeno", "Tópico", "titulo h3 heading topico", (e) => e.chain().focus().setHeading({ level: 3 }).run()),
    block("ul", "Lista", "Com marcadores", "lista bullet marcadores ul", (e) => e.chain().focus().toggleBulletList().run()),
    block("ol", "Lista numerada", "1, 2, 3…", "lista numerada ordenada ol", (e) => e.chain().focus().toggleOrderedList().run()),
    block("todo", "Checklist", "Tarefas para marcar", "checklist tarefa todo check", (e) => e.chain().focus().toggleTaskList().run()),
    block("quote", "Citação", "Trecho em destaque", "citacao quote blockquote", (e) => e.chain().focus().toggleBlockquote().run()),
    block("code", "Bloco de código", "Com cores de sintaxe", "codigo code bloco", (e) => e.chain().focus().toggleCodeBlock().run()),
    block("table", "Tabela", "3 × 3 com cabeçalho", "tabela table", (e) =>
      e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
    ),
    block("image", "Imagem", "Do computador", "imagem foto image figura", (e) => onImage(e)),
    block("hr", "Divisor", "Linha separando partes", "divisor linha separador hr", (e) => e.chain().focus().setHorizontalRule().run()),
  ];
}

export function SlashCommand(store: SlashStore, items: SlashItem[]) {
  return Extension.create({
    name: "slashCommand",
    addProseMirrorPlugins() {
      return [
        Suggestion<SlashItem, SlashItem>({
          editor: this.editor,
          char: "/",
          allowSpaces: false,
          items: ({ query }) => {
            const q = normalize(query);
            return items.filter((item) => !q || normalize(`${item.label} ${item.keywords}`).includes(q));
          },
          command: ({ editor, range, props }) => props.run(editor, range),
          render: () => {
            let current: SuggestionProps<SlashItem, SlashItem> | null = null;
            const show = (props: SuggestionProps<SlashItem, SlashItem>, index: number) => {
              current = props;
              store.set({
                open: props.items.length > 0,
                items: props.items,
                index: Math.min(index, Math.max(0, props.items.length - 1)),
                rect: props.clientRect?.() ?? null,
                pick: (item) => props.command(item),
              });
            };
            return {
              onStart: (props) => show(props, 0),
              onUpdate: (props) => show(props, 0),
              onKeyDown: ({ event }) => {
                const state = store.get();
                if (!state.open || !current) return false;
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  const step = event.key === "ArrowDown" ? 1 : -1;
                  const n = state.items.length;
                  store.set({ ...state, index: (state.index + step + n) % n });
                  return true;
                }
                if (event.key === "Enter" || event.key === "Tab") {
                  const item = state.items[state.index];
                  if (item) current.command(item);
                  return true;
                }
                if (event.key === "Escape") {
                  store.close();
                  return true;
                }
                return false;
              },
              onExit: () => {
                current = null;
                store.close();
              },
            };
          },
        }),
      ];
    },
  });
}
