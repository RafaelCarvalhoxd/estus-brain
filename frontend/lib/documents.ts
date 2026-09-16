import "server-only";

// Mirrors backend/internal/httpapi/dto_documents.go by hand, same convention
// as the other modules. The bytes of a document never pass through here:
// uploads stream to the Go API as multipart, and downloads are proxied by
// the route handler under app/api/documents.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface DocumentFolder {
  id: string;
  parent_id?: string;
  name: string;
  created_at: string;
}

export interface StoredDocument {
  id: string;
  folder_id?: string;
  name: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api ${path} -> ${res.status}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function listDocumentFolders(): Promise<DocumentFolder[]> {
  return apiFetch<DocumentFolder[]>("/api/document-folders", { cache: "no-store" });
}

export function listDocuments(folderId: string | null): Promise<StoredDocument[]> {
  const query = folderId ? `?folder_id=${encodeURIComponent(folderId)}` : "";
  return apiFetch<StoredDocument[]>(`/api/documents${query}`, { cache: "no-store" });
}

export function countDocuments(): Promise<{ count: number }> {
  return apiFetch<{ count: number }>("/api/documents/count", { cache: "no-store" });
}

export function createDocumentFolder(parentId: string | null, name: string): Promise<DocumentFolder> {
  return apiFetch<DocumentFolder>("/api/document-folders", {
    method: "POST",
    body: JSON.stringify({ parent_id: parentId, name }),
  });
}

export function renameDocumentFolder(id: string, name: string): Promise<DocumentFolder> {
  return apiFetch<DocumentFolder>(`/api/document-folders/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}

export function deleteDocumentFolder(id: string): Promise<void> {
  return apiFetch<void>(`/api/document-folders/${id}`, { method: "DELETE" });
}

// Multipart, so no JSON content-type: fetch has to set its own boundary.
export async function uploadDocument(file: File, folderId: string | null): Promise<StoredDocument> {
  const form = new FormData();
  form.append("file", file, file.name);
  if (folderId) form.append("folder_id", folderId);

  const res = await fetch(`${API_URL}/api/documents`, { method: "POST", body: form });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`estus-vault api /api/documents -> ${res.status}: ${body}`);
  }
  return res.json() as Promise<StoredDocument>;
}

export function moveDocument(id: string, folderId: string | null, name: string): Promise<StoredDocument> {
  return apiFetch<StoredDocument>(`/api/documents/${id}`, {
    method: "PUT",
    body: JSON.stringify({ folder_id: folderId, name }),
  });
}

export function deleteDocument(id: string): Promise<void> {
  return apiFetch<void>(`/api/documents/${id}`, { method: "DELETE" });
}
