import type { NextRequest } from "next/server";

// The chat screen talks to the assistant through here — the browser never
// reaches the Go API. Bodies stream both ways, so an AI answer arrives as it
// is written (server-sent events pass straight through).
const API_URL = process.env.API_URL ?? "http://localhost:8080";

async function proxy(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  const target = `${API_URL}/api/assistant/${path.map(encodeURIComponent).join("/")}${request.nextUrl.search}`;
  const hasBody = request.method !== "GET" && request.method !== "HEAD";
  const res = await fetch(target, {
    method: request.method,
    headers: {
      "Content-Type": request.headers.get("content-type") ?? "application/json",
      // A recording's format travels in its name.
      ...(request.headers.get("x-filename") ? { "X-Filename": request.headers.get("x-filename")! } : {}),
    },
    body: hasBody ? request.body : undefined,
    // Required by fetch to stream a request body.
    ...(hasBody ? { duplex: "half" } : {}),
    cache: "no-store",
    signal: request.signal,
  } as RequestInit);

  const headers = new Headers();
  for (const name of ["content-type", "cache-control"]) {
    const value = res.headers.get(name);
    if (value) headers.set(name, value);
  }
  headers.set("X-Accel-Buffering", "no");
  return new Response(res.body, { status: res.status, headers });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const DELETE = proxy;
