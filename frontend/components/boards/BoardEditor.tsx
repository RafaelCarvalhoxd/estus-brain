"use client";

import dynamic from "next/dynamic";

// Excalidraw touches window as soon as it's imported, so the editor is only
// ever loaded in the browser.
const BoardCanvas = dynamic(() => import("./BoardCanvas"), {
  ssr: false,
  loading: () => <div className="board-loading">Abrindo quadro…</div>,
});

export function BoardEditor(props: { boardId: string; name: string; scene: Record<string, unknown> }) {
  return (
    <div className="board-stage">
      <BoardCanvas {...props} />
    </div>
  );
}
