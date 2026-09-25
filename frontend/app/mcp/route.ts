import type { NextRequest } from "next/server";

// The MCP endpoint, published on the app's own address so the external
// agent behind the chat, and any other MCP client outside the server
// (Claude Desktop through estus-mcp, Claude.ai, ChatGPT…), reach it through
// the same edge as the app. The Go server checks the bearer token.
const API_URL = process.env.API_URL ?? "http://localhost:8080";
// Every Mcp-* header goes through: newer clients send Mcp-Method, Mcp-Name
// and Mcp-Param-*, and the Go server refuses a request that lacks them.
const FORWARDED = ["authorization", "content-type", "accept", "last-event-id"];

async function proxy(request: NextRequest) {
  const headers = new Headers();
  request.headers.forEach((value, name) => {
    if (FORWARDED.includes(name) || name.startsWith("mcp-")) headers.set(name, value);
  });
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
