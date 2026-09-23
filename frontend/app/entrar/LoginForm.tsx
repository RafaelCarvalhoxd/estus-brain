"use client";

import { useActionState } from "react";
import { loginAction, type LoginState } from "./actions";

export function LoginForm({ from }: { from: string }) {
  const [state, formAction, pending] = useActionState<LoginState, FormData>(loginAction, {});
  return (
    <form action={formAction} className="form-grid">
      <input type="hidden" name="de" value={from} />
      <div className="field">
        <label htmlFor="login-password">Senha</label>
        <input id="login-password" name="password" type="password" autoComplete="current-password" autoFocus required />
      </div>
      <button className="btn-block" type="submit" disabled={pending}>
        {pending ? "Entrando…" : "Entrar"}
      </button>
      {state.error && <p className="form-error">{state.error}</p>}
    </form>
  );
}
