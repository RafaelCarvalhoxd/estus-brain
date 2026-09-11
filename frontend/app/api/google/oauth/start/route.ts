import { NextResponse } from "next/server";

// Per the architecture rule, the browser never talks to the Go backend
// directly — every request goes through the Next.js server. The Go
// handler for GET /api/google/oauth/start responds with an HTTP redirect
// (Location: Google's consent URL) rather than JSON, so this route fetches
// it server-side with redirect: "manual", reads that Location header, and
// hands the browser a redirect straight to Google. The browser never sees
// the Go backend's address.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export async function GET(request: Request) {
  const res = await fetch(`${API_URL}/api/google/oauth/start`, { redirect: "manual" });

  const location = res.headers.get("location");
  if (res.status >= 300 && res.status < 400 && location) {
    return NextResponse.redirect(location);
  }

  // The backend answers with a JSON error (not a redirect) when Google
  // OAuth env vars aren't configured — bounce back to /agenda so the page
  // can show its own "not connected" state instead of a raw error page.
  return NextResponse.redirect(new URL("/agenda?google_error=1", request.url));
}
