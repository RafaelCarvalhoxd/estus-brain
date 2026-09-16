"use client";

import { useActionState, useState, useTransition } from "react";
import Link from "next/link";
import type { BoardSummary } from "@/lib/boards";
import { createBoardAction, deleteBoardAction, renameBoardAction, type BoardFormState } from "@/app/quadros/actions";
import { IconFlow, IconPencil, IconPlus, IconTrash } from "./icons";

const IDLE: BoardFormState = { status: "idle" };

function formatEdited(iso: string): string {
  const date = new Date(iso);
  const minutes = Math.round((Date.now() - date.getTime()) / 60000);
  if (minutes < 1) return "Editado agora";
  if (minutes < 60) return `Editado há ${minutes} min`;
  if (minutes < 60 * 24) return `Editado há ${Math.round(minutes / 60)} h`;
  return `Editado em ${date.toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" })}`;
}

// The SVG preview is rendered by the editor on save. Loaded through <img>,
// so whatever it contains can't run anything.
function previewSrc(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function BoardsGallery({ boards }: { boards: BoardSummary[] }) {
  const [createState, create, creating] = useActionState(createBoardAction, IDLE);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const run = (fn: () => Promise<{ error?: string }>) => {
    setError(null);
    startTransition(async () => {
      const result = await fn();
      if (result.error) setError(result.error);
    });
  };

  const rename = (board: BoardSummary) => {
    const next = window.prompt("Novo nome", board.name);
    if (next === null || next.trim() === board.name) return;
    run(() => renameBoardAction(board.id, next));
  };

  const remove = (board: BoardSummary) => {
    if (!window.confirm(`Excluir "${board.name}"? O desenho será perdido.`)) return;
    run(() => deleteBoardAction(board.id));
  };

  return (
    <>
      <form action={create} className="board-new">
        <input name="name" placeholder="Nome do quadro" aria-label="Nome do novo quadro" maxLength={120} />
        <button className="btn-primary" type="submit" disabled={creating}>
          <IconPlus />
          {creating ? "Criando…" : "Novo quadro"}
        </button>
      </form>

      {createState.status === "error" && <p className="form-error">{createState.message}</p>}
      {error && <p className="form-error">{error}</p>}

      {boards.length === 0 ? (
        <div className="panel board-empty">
          <span className="board-empty-icon">
            <IconFlow />
          </span>
          <p>Nenhum quadro ainda. Crie um para desenhar fluxos, diagramas e rascunhos.</p>
        </div>
      ) : (
        <div className="board-grid">
          {boards.map((b) => (
            <article className="panel board-card" key={b.id}>
              <Link href={`/quadros/${b.id}`} className="board-open" aria-label={`Abrir ${b.name}`}>
                <div className="board-thumb">
                  {b.preview ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={previewSrc(b.preview)} alt="" />
                  ) : (
                    <span className="board-thumb-empty">Vazio</span>
                  )}
                </div>
              </Link>
              <div className="board-foot">
                <div className="board-meta">
                  <Link href={`/quadros/${b.id}`} className="board-name">
                    {b.name}
                  </Link>
                  <span className="board-edited">{formatEdited(b.updated_at)}</span>
                </div>
                <div className="row-actions">
                  <button
                    className="icon-btn"
                    type="button"
                    aria-label={`Renomear ${b.name}`}
                    disabled={pending}
                    onClick={() => rename(b)}
                  >
                    <IconPencil />
                  </button>
                  <button
                    className="icon-btn bad"
                    type="button"
                    aria-label={`Excluir ${b.name}`}
                    disabled={pending}
                    onClick={() => remove(b)}
                  >
                    <IconTrash />
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </>
  );
}
