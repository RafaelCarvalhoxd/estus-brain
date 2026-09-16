import type { NextRequest } from "next/server";

// The MCP endpoint, published on the app's own address so AI apps outside
// the server (Claude Desktop through estus-mcp, Claude Code, Codex…) reach it
// through the same edge as the app. The Go server checks the bearer token.
const API_URL = process.env.API_URL ?? "http://localhost:8080";
const FORWARDED = ["authorization", "content-type", "accept", "mcp-session-id", "mcp-protocol-version", "last-event-id"];

async function proxy(request: NextRequest) {
  const headers = new Headers();
  for (const name of FORWARDED) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  const hasBody = request.method === "POST";
  const res = await fetch(`${API_URL}/mcp${request.nextUrl.search}`, {
    method: request.method,
    headers,
    body: hasBody ? request.body : undefined,
    ...(hasBody ? { duplex: "half" } : {}),
    cache: "no-store",
    signal: request.signal,
  } as RequestInit);

  const out = new Headers();
  for (const name of ["content-type", "mcp-session-id", "www-authenticate", "cache-control"]) {
    const value = res.headers.get(name);
    if (value) out.set(name, value);
  }
  return new Response(res.body, { status: res.status, headers: out });
}

export const GET = proxy;
export const POST = proxy;
export const DELETE = proxy;
