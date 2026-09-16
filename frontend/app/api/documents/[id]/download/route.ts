import { NextResponse } from "next/server";

// The browser never talks to the Go API directly, so a download is proxied
// here: the file streams through this handler rather than being buffered,
// and the API stays unreachable from the outside.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const res = await fetch(`${API_URL}/api/documents/${id}/content`, { cache: "no-store" });

  if (!res.ok || !res.body) {
    return NextResponse.json({ error: "Arquivo não encontrado." }, { status: res.status === 404 ? 404 : 502 });
  }

  const headers = new Headers();
  for (const header of ["content-type", "content-length", "content-disposition"]) {
    const value = res.headers.get(header);
    if (value) headers.set(header, value);
  }
  return new NextResponse(res.body, { status: 200, headers });
}
