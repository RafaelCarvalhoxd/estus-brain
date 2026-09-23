"use server";

import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE, SESSION_MAX_AGE, checkPassword, newSessionToken } from "@/lib/session";

export type LoginState = { error?: string };

export async function loginAction(_prev: LoginState, formData: FormData): Promise<LoginState> {
  const password = String(formData.get("password") ?? "");
  if (!checkPassword(password)) {
    // Slows down guessing; the app has a single owner, so nobody waits on this.
    await new Promise((r) => setTimeout(r, 1000));
    return { error: process.env.APP_PASSWORD ? "Senha incorreta." : "APP_PASSWORD não está configurada no servidor." };
  }

  const secure = (await headers()).get("x-forwarded-proto") === "https";
  (await cookies()).set(SESSION_COOKIE, newSessionToken(), {
    httpOnly: true,
    sameSite: "lax",
    secure,
    path: "/",
    maxAge: SESSION_MAX_AGE,
  });

  const from = String(formData.get("de") ?? "");
  // Only paths on this site, never "//other-host".
  redirect(from.startsWith("/") && !from.startsWith("//") ? from : "/");
}
