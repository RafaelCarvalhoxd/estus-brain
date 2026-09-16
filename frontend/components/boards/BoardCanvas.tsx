"use client";

import "./excalidraw-assets";
import "@excalidraw/excalidraw/index.css";
import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { Excalidraw, MainMenu, WelcomeScreen, exportToSvg, hashElementsVersion, serializeAsJSON } from "@excalidraw/excalidraw";
import type { AppState, BinaryFiles, ExcalidrawInitialDataState, LibraryItems } from "@excalidraw/excalidraw/types";
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";

type SaveStatus = "saved" | "dirty" | "saving" | "error";

const SAVE_DELAY_MS = 1200;
const MAX_PREVIEW_CHARS = 500_000;
const LIBRARY_KEY = "estus-board-library";
const UI_OPTIONS = { canvasActions: { toggleTheme: false } };
const STATUS_LABEL: Record<SaveStatus, string> = {
  saved: "Salvo",
  dirty: "Alterações pendentes",
  saving: "Salvando…",
  error: "Erro ao salvar",
};

interface Snapshot {
  elements: readonly ExcalidrawElement[];
  appState: AppState;
  files: BinaryFiles;
}

// Cheap enough to run on every change event (they fire on pointer moves
// too): only a real edit to the drawing, a new image or a new background
// changes it, so panning and selecting never trigger a save.
function signatureOf({ elements, appState, files }: Snapshot): string {
  return `${hashElementsVersion(elements)}:${Object.keys(files).length}:${appState.viewBackgroundColor}`;
}

async function renderPreview(snapshot: Snapshot, elements: readonly ExcalidrawElement[]): Promise<string> {
  if (elements.length === 0) return "";
  const opts = {
    elements,
    appState: { ...snapshot.appState, exportBackground: false, exportWithDarkMode: false },
    files: snapshot.files,
    exportPadding: 24,
  };
  // Inlined fonts keep handwriting in the thumbnail; fall back to none when
  // that (or pasted images) makes it too heavy for a list.
  let svg = (await exportToSvg(opts)).outerHTML;
  if (svg.length > MAX_PREVIEW_CHARS) svg = (await exportToSvg({ ...opts, skipInliningFonts: true })).outerHTML;
  return svg.length > MAX_PREVIEW_CHARS ? "" : svg;
}

function readTheme(): "light" | "dark" {
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

function readLibrary(): LibraryItems {
  try {
    const raw = localStorage.getItem(LIBRARY_KEY);
    return raw ? (JSON.parse(raw) as LibraryItems) : [];
  } catch {
    return [];
  }
}

interface SaveState {
  status: SaveStatus;
  error: string | null;
}

// The autosave lives outside React: it's a small state machine fed by the
// editor's change events, one per mounted board. Only a real edit arms the
// timer; one save runs at a time, and changes made during it queue another.
// Its status is a store of its own, read only by the indicator — the editor
// reports changes while it renders, so re-rendering it from here would loop.
function createAutosave(boardId: string) {
  let state: SaveState = { status: "saved", error: null };
  const listeners = new Set<() => void>();
  const report = (status: SaveStatus, error: string | null) => {
    if (state.status === status && state.error === error) return;
    state = { status, error };
    listeners.forEach((l) => l());
  };

  let latest: Snapshot | null = null;
  let savedSignature: string | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let inFlight = false;
  let queued = false;

  // Resolves to whether anything was written.
  async function save(): Promise<boolean> {
    if (timer) clearTimeout(timer);
    timer = null;
    const snapshot = latest;
    if (!snapshot) return false;
    if (inFlight) {
      queued = true;
      return false;
    }

    const signature = signatureOf(snapshot);
    if (signature === savedSignature) {
      report("saved", null);
      return false;
    }

    inFlight = true;
    report("saving", null);
    try {
      const elements = snapshot.elements.filter((el) => !el.isDeleted);
      const body = JSON.stringify({
        scene: JSON.parse(serializeAsJSON(elements, snapshot.appState, snapshot.files, "local")),
        preview: await renderPreview(snapshot, elements),
      });
      const res = await fetch(`/api/boards/${boardId}/scene`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body,
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(data?.error ?? "Não foi possível salvar o quadro.");
      }
      savedSignature = signature;
      report(latest && signatureOf(latest) !== signature ? "dirty" : "saved", null);
      return true;
    } catch (err) {
      report("error", err instanceof Error ? err.message : "Não foi possível salvar o quadro.");
      return false;
    } finally {
      inFlight = false;
      if (queued) {
        queued = false;
        void save();
      }
    }
  }

  return {
    save,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    getState: () => state,
    change(snapshot: Snapshot) {
      latest = snapshot;
      const signature = signatureOf(snapshot);
      // The first change event is the loaded scene itself.
      if (savedSignature === null) {
        savedSignature = signature;
        return;
      }
      if (signature === savedSignature) return;
      if (!inFlight) report("dirty", null);
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => void save(), SAVE_DELAY_MS);
    },
    pending(): boolean {
      return inFlight || (latest !== null && signatureOf(latest) !== savedSignature);
    },
  };
}

export default function BoardCanvas({ boardId, name, scene }: { boardId: string; name: string; scene: Record<string, unknown> }) {
  const [theme, setTheme] = useState<"light" | "dark">("dark");
  const router = useRouter();
  const [autosave] = useState(() => createAutosave(boardId));

  // Read once: Excalidraw only takes its initial scene on mount.
  const [initialData] = useState<ExcalidrawInitialDataState>(() => ({
    ...(scene as ExcalidrawInitialDataState),
    libraryItems: readLibrary(),
    scrollToContent: true,
  }));

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTheme(readTheme());
    const observer = new MutationObserver(() => setTheme(readTheme()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => observer.disconnect();
  }, []);

  const onChange = useCallback(
    (elements: readonly ExcalidrawElement[], appState: AppState, files: BinaryFiles) =>
      autosave.change({ elements, appState, files }),
    [autosave],
  );

  // Leaving the board (the breadcrumb, the back button) saves whatever is
  // pending, then refreshes the screen you landed on so its thumbnail is the
  // latest; closing the tab with unsaved work asks first.
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (!autosave.pending()) return;
      void autosave.save();
      e.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
      void autosave.save().then((saved) => saved && router.refresh());
    };
  }, [autosave, router]);

  const onLibraryChange = useCallback((items: LibraryItems) => {
    try {
      localStorage.setItem(LIBRARY_KEY, JSON.stringify(items));
    } catch {
      // Storage full or blocked — the library just won't persist.
    }
  }, []);

  // On a phone the editor puts this corner inside its toolbar, which has no
  // room to spare; saving carries on without the indicator.
  const renderTopRightUI = useCallback(
    (isMobile: boolean) => (isMobile ? null : <SaveIndicator autosave={autosave} />),
    [autosave],
  );

  return (
    <Excalidraw
      initialData={initialData}
      onChange={onChange}
      onLibraryChange={onLibraryChange}
      theme={theme}
      name={name}
      langCode="pt-BR"
      aiEnabled={false}
      UIOptions={UI_OPTIONS}
      renderTopRightUI={renderTopRightUI}
    >
      <MainMenu>
        <MainMenu.DefaultItems.LoadScene />
        <MainMenu.DefaultItems.Export />
        <MainMenu.DefaultItems.SaveAsImage />
        <MainMenu.DefaultItems.SearchMenu />
        <MainMenu.DefaultItems.CommandPalette />
        <MainMenu.DefaultItems.Help />
        <MainMenu.DefaultItems.ClearCanvas />
        <MainMenu.Separator />
        <MainMenu.DefaultItems.ChangeCanvasBackground />
      </MainMenu>
      <WelcomeScreen>
        <WelcomeScreen.Hints.MenuHint />
        <WelcomeScreen.Hints.ToolbarHint />
        <WelcomeScreen.Hints.HelpHint />
        <WelcomeScreen.Center>
          <WelcomeScreen.Center.Heading>Desenhe fluxos, diagramas e ideias. Tudo é salvo sozinho.</WelcomeScreen.Center.Heading>
        </WelcomeScreen.Center>
      </WelcomeScreen>
    </Excalidraw>
  );
}

function SaveIndicator({ autosave }: { autosave: ReturnType<typeof createAutosave> }) {
  const { status, error } = useSyncExternalStore(autosave.subscribe, autosave.getState, autosave.getState);
  return (
    <button
      type="button"
      className={`board-status is-${status}`}
      title={error ?? STATUS_LABEL[status]}
      onClick={() => void autosave.save()}
      disabled={status === "saving" || status === "saved"}
    >
      <span className="board-status-dot" aria-hidden="true" />
      <span className="board-status-label">{STATUS_LABEL[status]}</span>
    </button>
  );
}
