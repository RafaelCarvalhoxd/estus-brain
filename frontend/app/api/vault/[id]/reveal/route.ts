import { NextResponse } from "next/server";

const API_URL = process.env.API_URL ?? "http://localhost:8080";

// The body carries the app password; the Go server checks it before
// decrypting anything.
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  const res = await fetch(`${API_URL}/api/vault/${id}/reveal`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: await request.text(),
  });
  const responseBody = await res.text();
  return new NextResponse(responseBody, {
    status: res.status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}
