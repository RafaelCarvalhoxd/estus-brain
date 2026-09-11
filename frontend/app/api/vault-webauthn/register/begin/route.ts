import { NextResponse } from "next/server";

// Thin proxy: the browser calls this directly (never the Go backend) so
// that navigator.credentials.create() can run client-side while the Go
// service still has no public listener of its own.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function POST() {
  const res = await fetch(`${API_URL}/api/vault/webauthn/register/begin`, { method: "POST" });
  const body = await res.text();
  return new NextResponse(body, {
    status: res.status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}
