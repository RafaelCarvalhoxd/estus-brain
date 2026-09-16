"use client";

import { useActionState, useRef, useState, useTransition } from "react";
import Link from "next/link";
import type { DocumentFolder, StoredDocument } from "@/lib/documents";
import {
  createFolderAction,
  deleteDocumentAction,
  deleteFolderAction,
  renameDocumentAction,
  renameFolderAction,
  uploadDocumentsAction,
  type DocumentFormState,
} from "@/app/documentos/actions";
import { IconDownload, IconFile, IconFolder, IconPencil, IconTrash, IconUpload } from "./icons";

const IDLE: DocumentFormState = { status: "idle" };

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" });
}

export function DocumentsBrowser({
  folderId,
  folders,
  documents,
}: {
  folderId: string | null;
  folders: DocumentFolder[];
  documents: StoredDocument[];
}) {
  const [uploadState, upload, uploading] = useActionState(uploadDocumentsAction, IDLE);
  const [folderState, createFolder, creatingFolder] = useActionState(createFolderAction, IDLE);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const fileInput = useRef<HTMLInputElement>(null);

  const run = (fn: () => Promise<{ error?: string }>) => {
    setError(null);
    startTransition(async () => {
      const result = await fn();
      if (result.error) setError(result.error);
    });
  };

  const rename = (current: string, apply: (next: string) => Promise<{ error?: string }>) => {
    const next = window.prompt("Novo nome", current);
    if (next === null || next.trim() === current) return;
    run(() => apply(next));
  };

  return (
    <>
      <div className="doc-actions">
        <form action={upload} className="doc-upload">
          <input type="hidden" name="folder_id" value={folderId ?? ""} />
          <input ref={fileInput} type="file" name="file" id="doc-file" multiple required />
          <label className="btn-outline" htmlFor="doc-file">
            Escolher arquivos
          </label>
          <button className="btn-primary" type="submit" disabled={uploading}>
            <IconUpload />
            {uploading ? "Enviando…" : "Enviar"}
          </button>
        </form>

        <form action={createFolder} className="doc-new-folder">
          <input type="hidden" name="folder_id" value={folderId ?? ""} />
          <input name="name" placeholder="Nova pasta" aria-label="Nome da nova pasta" maxLength={120} />
          <button className="btn-outline" type="submit" disabled={creatingFolder}>
            Criar pasta
          </button>
        </form>
      </div>

      {uploadState.status !== "idle" && (
        <p className={uploadState.status === "error" ? "form-error" : "form-success"}>{uploadState.message}</p>
      )}
      {folderState.status !== "idle" && (
        <p className={folderState.status === "error" ? "form-error" : "form-success"}>{folderState.message}</p>
      )}
      {error && <p className="form-error">{error}</p>}

      {folders.length > 0 && (
        <div className="doc-folders">
          {folders.map((f) => (
            <div className="doc-folder" key={f.id}>
              <Link href={`/documentos?folder=${f.id}`} className="doc-folder-open">
                <span className="doc-folder-icon">
                  <IconFolder />
                </span>
                <span className="doc-folder-name">{f.name}</span>
              </Link>
              <div className="row-actions">
                <button
                  className="icon-btn"
                  type="button"
                  aria-label={`Renomear ${f.name}`}
                  disabled={pending}
                  onClick={() => rename(f.name, (next) => renameFolderAction(f.id, next))}
                >
                  <IconPencil />
                </button>
                <button
                  className="icon-btn bad"
                  type="button"
                  aria-label={`Excluir ${f.name}`}
                  disabled={pending}
                  onClick={() => run(() => deleteFolderAction(f.id))}
                >
                  <IconTrash />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="panel">
        <div className="panel-head">
          <h2>Arquivos</h2>
          <span>{documents.length === 1 ? "1 arquivo" : `${documents.length} arquivos`}</span>
        </div>

        {documents.length === 0 ? (
          <p className="empty-note">Nenhum arquivo nesta pasta. Envie o primeiro acima.</p>
        ) : (
          <div className="doc-list">
            {documents.map((d) => (
              <div className="doc-row" key={d.id}>
                <span className="doc-row-icon">
                  <IconFile />
                </span>
                <div className="doc-row-main">
                  <span className="doc-row-name">{d.name}</span>
                  <span className="doc-row-meta">
                    {formatSize(d.size_bytes)} · {formatDate(d.created_at)}
                  </span>
                </div>
                <div className="row-actions">
                  <a
                    className="icon-btn"
                    href={`/api/documents/${d.id}/download`}
                    aria-label={`Baixar ${d.name}`}
                    download={d.name}
                  >
                    <IconDownload />
                  </a>
                  <button
                    className="icon-btn"
                    type="button"
                    aria-label={`Renomear ${d.name}`}
                    disabled={pending}
                    onClick={() => rename(d.name, (next) => renameDocumentAction(d.id, folderId, next))}
                  >
                    <IconPencil />
                  </button>
                  <button
                    className="icon-btn bad"
                    type="button"
                    aria-label={`Excluir ${d.name}`}
                    disabled={pending}
                    onClick={() => run(() => deleteDocumentAction(d.id))}
                  >
                    <IconTrash />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
