"use client";

import { useActionState, useEffect, useRef, useState, useTransition } from "react";
import { createPortal } from "react-dom";
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
import { IconClose, IconDownload, IconFile, IconFolder, IconPaperclip, IconPencil, IconPlus, IconTrash } from "./icons";

const IDLE: DocumentFormState = { status: "idle" };

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" });
}

// Finder's order: "Nota 2" before "Nota 10", accents and case ignored.
const byName = new Intl.Collator("pt-BR", { numeric: true, sensitivity: "base" });

const KIND_BY_TYPE: Record<string, string> = {
  "application/pdf": "Documento PDF",
  "text/markdown": "Markdown",
  "text/plain": "Texto",
  "text/csv": "Planilha CSV",
  "application/json": "JSON",
  "application/zip": "Arquivo ZIP",
};

// A short "Tipo" column like Finder's: known types by name, the rest by
// family or extension.
function kindLabel(doc: StoredDocument): string {
  const type = doc.content_type.split(";")[0].trim().toLowerCase();
  const ext = doc.name.includes(".") ? doc.name.split(".").pop()!.toUpperCase() : "";
  if (KIND_BY_TYPE[type]) return KIND_BY_TYPE[type];
  if (type.startsWith("image/")) return ext ? `Imagem ${ext}` : "Imagem";
  if (type.startsWith("video/")) return ext ? `Vídeo ${ext}` : "Vídeo";
  if (type.startsWith("audio/")) return ext ? `Áudio ${ext}` : "Áudio";
  return ext ? `Documento ${ext}` : "Documento";
}

type PreviewKind = "image" | "video" | "audio" | "frame" | null;

// What the browser can show on its own. Anything else (zip, docx, xlsx…)
// has no built-in viewer and only offers the download.
function previewKind(contentType: string): PreviewKind {
  const type = contentType.split(";")[0].trim().toLowerCase();
  if (type.startsWith("image/")) return "image";
  if (type.startsWith("video/")) return "video";
  if (type.startsWith("audio/")) return "audio";
  if (type === "application/pdf" || type.startsWith("text/") || type === "application/json") return "frame";
  return null;
}

function DocumentViewer({ doc, onClose }: { doc: StoredDocument; onClose: () => void }) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const src = `/api/documents/${doc.id}/download?inline=1`;
  const kind = previewKind(doc.content_type);

  // Portaled for the same reason as Modal: a panel's backdrop-filter would
  // trap position: fixed inside it.
  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div className="doc-viewer" role="dialog" aria-label={doc.name} onClick={(e) => e.stopPropagation()}>
        <div className="doc-viewer-head">
          <span className="doc-viewer-name">{doc.name}</span>
          <a className="icon-btn" href={`/api/documents/${doc.id}/download`} download={doc.name} aria-label={`Baixar ${doc.name}`}>
            <IconDownload />
          </a>
          <button className="icon-btn" type="button" aria-label="Fechar" onClick={onClose}>
            <IconClose />
          </button>
        </div>
        <div className="doc-viewer-body">
          {kind === "image" && (
            // A file of any size from the vault, not a static asset next/image could optimize.
            // eslint-disable-next-line @next/next/no-img-element
            <img src={src} alt={doc.name} />
          )}
          {kind === "video" && <video src={src} controls autoPlay />}
          {kind === "audio" && <audio src={src} controls autoPlay />}
          {kind === "frame" && <iframe src={src} title={doc.name} />}
          {kind === null && (
            <div className="doc-viewer-empty">
              <p>Não dá para pré-visualizar este tipo de arquivo.</p>
              <a className="btn-primary" href={`/api/documents/${doc.id}/download`} download={doc.name}>
                <IconDownload />
                Baixar
              </a>
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
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
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const [draftFolder, setDraftFolder] = useState(false);
  const [viewing, setViewing] = useState<StoredDocument | null>(null);
  const uploadForm = useRef<HTMLFormElement>(null);
  // Enter saves and then the input unmounts, which fires blur: without this
  // the folder would be created twice.
  const savingFolder = useRef(false);

  const sortedFolders = [...folders].sort((a, b) => byName.compare(a.name, b.name));
  const sortedDocuments = [...documents].sort((a, b) => byName.compare(a.name, b.name));

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

  const saveDraftFolder = (name: string) => {
    if (savingFolder.current) return;
    savingFolder.current = true;
    setDraftFolder(false);
    if (!name.trim()) {
      savingFolder.current = false;
      return;
    }
    run(async () => {
      const result = await createFolderAction(folderId, name);
      savingFolder.current = false;
      return result;
    });
  };

  return (
    <>
      <div className="doc-actions">
        <form ref={uploadForm} action={upload} className="doc-upload">
          <input type="hidden" name="folder_id" value={folderId ?? ""} />
          <input
            type="file"
            name="file"
            id="doc-file"
            multiple
            disabled={uploading}
            onChange={(e) => {
              if (e.currentTarget.files?.length) uploadForm.current?.requestSubmit();
            }}
          />
          <label className={`btn-primary${uploading ? " is-busy" : ""}`} htmlFor="doc-file" aria-disabled={uploading}>
            <IconPaperclip />
            {uploading ? "Enviando…" : "Anexar arquivo"}
          </label>
        </form>

        <button className="btn-outline doc-new-folder" type="button" disabled={draftFolder} onClick={() => setDraftFolder(true)}>
          <IconPlus />
          Criar pasta
        </button>
      </div>

      {uploadState.status !== "idle" && (
        <p className={uploadState.status === "error" ? "form-error" : "form-success"}>{uploadState.message}</p>
      )}
      {error && <p className="form-error">{error}</p>}

      <div className="panel doc-finder">
        <div className="doc-finder-head" aria-hidden="true">
          <span>Nome</span>
          <span>Criado</span>
          <span>Tamanho</span>
          <span>Tipo</span>
          <span />
        </div>

        {draftFolder && (
          <div className="doc-item is-draft">
            <span className="doc-item-name">
              <span className="doc-item-icon is-folder">
                <IconFolder />
              </span>
              <input
                className="doc-folder-input"
                aria-label="Nome da nova pasta"
                placeholder="Nome da pasta"
                maxLength={120}
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter") saveDraftFolder(e.currentTarget.value);
                  if (e.key === "Escape") setDraftFolder(false);
                }}
                onBlur={(e) => saveDraftFolder(e.currentTarget.value)}
              />
            </span>
          </div>
        )}

        {sortedFolders.map((f) => (
          <div className="doc-item" key={f.id}>
            <Link href={`/documentos?folder=${f.id}`} className="doc-item-name">
              <span className="doc-item-icon is-folder">
                <IconFolder />
              </span>
              <span className="doc-item-label">{f.name}</span>
            </Link>
            <span className="doc-item-cell">{formatDate(f.created_at)}</span>
            <span className="doc-item-cell">—</span>
            <span className="doc-item-cell">Pasta</span>
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

        {sortedDocuments.map((d) => (
          <div className="doc-item" key={d.id}>
            <button className="doc-item-name" type="button" onClick={() => setViewing(d)} aria-label={`Abrir ${d.name}`}>
              <span className="doc-item-icon">
                <IconFile />
              </span>
              <span className="doc-item-label">{d.name}</span>
            </button>
            <span className="doc-item-cell">{formatDate(d.created_at)}</span>
            <span className="doc-item-cell">{formatSize(d.size_bytes)}</span>
            <span className="doc-item-cell">{kindLabel(d)}</span>
            <div className="row-actions">
              <a className="icon-btn" href={`/api/documents/${d.id}/download`} aria-label={`Baixar ${d.name}`} download={d.name}>
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

        {!draftFolder && folders.length === 0 && documents.length === 0 && (
          <p className="empty-note">Pasta vazia. Anexe um arquivo ou crie uma pasta acima.</p>
        )}
      </div>

      {viewing && <DocumentViewer doc={viewing} onClose={() => setViewing(null)} />}
    </>
  );
}
