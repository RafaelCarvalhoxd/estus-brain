import { createHmac, timingSafeEqual } from "node:crypto";

// Login is one password, APP_PASSWORD, shared with the Go server (which
// asks for it again to reveal a vault password). The session cookie is an
// expiry signed with a key derived from that password, so changing the
// password logs every browser out.
export const SESSION_COOKIE = "estus_session";
export const SESSION_MAX_AGE = 60 * 60 * 24 * 30; // seconds

function appPassword(): string {
  return process.env.APP_PASSWORD ?? "";
}

function sign(payload: string): string {
  const key = createHmac("sha256", "estus-session").update(appPassword()).digest();
  return createHmac("sha256", key).update(payload).digest("base64url");
}

function safeEqual(a: string, b: string): boolean {
  const x = Buffer.from(a);
  const y = Buffer.from(b);
  return x.length === y.length && timingSafeEqual(x, y);
}

// An empty APP_PASSWORD never matches, so a server without it stays locked.
export function checkPassword(candidate: string): boolean {
  const expected = appPassword();
  return expected !== "" && safeEqual(candidate, expected);
}

export function newSessionToken(): string {
  const expires = String(Math.floor(Date.now() / 1000) + SESSION_MAX_AGE);
  return `${expires}.${sign(expires)}`;
}

export function isValidSession(token: string | undefined): boolean {
  if (!token || appPassword() === "") return false;
  const [expires, signature] = token.split(".");
  if (!expires || !signature || !safeEqual(signature, sign(expires))) return false;
  return Number(expires) > Date.now() / 1000;
}
