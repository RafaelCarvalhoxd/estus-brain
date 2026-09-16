import { NextResponse } from "next/server";

// The editor's autosave. The browser never talks to the Go API directly, and
// a note with pasted images is past the server-action body limit, so the save
// is proxied here. Only what the screen needs comes back, not the document.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function PUT(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const res = await fetch(`${API_URL}/api/notes/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: await request.text(),
    cache: "no-store",
  });

  if (res.ok) {
    const note = (await res.json()) as { updated_at: string };
    return NextResponse.json({ updated_at: note.updated_at });
  }

  let message = "Não foi possível salvar a nota.";
  if (res.status === 404) message = "Esta nota não existe mais.";
  if (res.status === 422) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    if (body?.error?.includes("larger than")) message = "A nota passou do limite de 10 MB. Remova algumas imagens.";
  }
  return NextResponse.json({ error: message }, { status: res.status === 404 || res.status === 422 ? res.status : 502 });
}
