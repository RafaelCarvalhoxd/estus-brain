import { NextResponse, type NextRequest } from "next/server";
import { SESSION_COOKIE, isValidSession } from "@/lib/session";

// Every page, route and server action needs the login cookie. /mcp is left
// out because the Go server checks its own bearer token there.
export function proxy(request: NextRequest) {
  if (isValidSession(request.cookies.get(SESSION_COOKIE)?.value)) return NextResponse.next();

  const { pathname, search } = request.nextUrl;
  if (pathname.startsWith("/api/")) {
    return NextResponse.json({ error: "login necessário" }, { status: 401 });
  }
  const login = new URL("/entrar", request.url);
  if (pathname !== "/") login.searchParams.set("de", pathname + search);
  return NextResponse.redirect(login);
}

export const config = {
  matcher: ["/((?!entrar|mcp|_next/static|_next/image|favicon.ico).*)"],
};
