import { NextResponse } from "next/server";

// The browser never talks to the Go API directly, so a download is proxied
// here: the file streams through this handler rather than being buffered,
// and the API stays unreachable from the outside.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
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

  // ?inline=1 is the viewer asking to show the file instead of saving it.
  if (new URL(request.url).searchParams.has("inline")) {
    const disposition = headers.get("content-disposition");
    headers.set("content-disposition", disposition ? disposition.replace(/^attachment/, "inline") : "inline");
    headers.set("x-content-type-options", "nosniff");
    // Markdown, CSV, JSON… would be offered as a download by some browsers;
    // as plain text every browser shows them, accents included.
    const type = headers.get("content-type") ?? "";
    if ((type.startsWith("text/") && !type.startsWith("text/html")) || type.startsWith("application/json")) {
      headers.set("content-type", "text/plain; charset=utf-8");
    }
    // An uploaded HTML or SVG would otherwise run its scripts on this
    // origin. Chrome refuses to render a PDF under a sandbox, and its PDF
    // viewer runs nothing on the page anyway, so PDFs are left out.
    if (!headers.get("content-type")?.startsWith("application/pdf")) {
      headers.set("content-security-policy", "sandbox");
    }
  }

  return new NextResponse(res.body, { status: 200, headers });
}
