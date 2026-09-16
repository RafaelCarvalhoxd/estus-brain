import { NextResponse } from "next/server";

// The editor's autosave. The browser never talks to the Go API directly, and
// a scene with pasted images is well past the server-action body limit, so
// the save is proxied here as-is. The API validates and caps the size.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function PUT(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const res = await fetch(`${API_URL}/api/boards/${encodeURIComponent(id)}/scene`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: await request.text(),
    cache: "no-store",
  });

  if (res.ok) return new NextResponse(null, { status: 204 });

  let message = "Não foi possível salvar o quadro.";
  if (res.status === 404) message = "Este quadro não existe mais.";
  if (res.status === 422) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    if (body?.error?.includes("larger than")) message = "O quadro passou do limite de 20 MB. Remova algumas imagens.";
  }
  return NextResponse.json({ error: message }, { status: res.status === 404 || res.status === 422 ? res.status : 502 });
}
