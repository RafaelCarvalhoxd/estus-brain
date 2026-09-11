import { NextResponse } from "next/server";

const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const session = new URL(request.url).searchParams.get("session") ?? "";
  const body = await request.text();

  const res = await fetch(
    `${API_URL}/api/vault/${id}/reveal/finish?session=${encodeURIComponent(session)}`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body },
  );
  const responseBody = await res.text();
  return new NextResponse(responseBody, {
    status: res.status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}
