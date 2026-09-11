"use client";

import { useActionState, useEffect } from "react";
import {
  createVaultEntryAction,
  updateVaultEntryAction,
  vaultFormInitialState,
  type VaultFormState,
} from "@/app/senhas/actions";
import type { VaultEntry } from "@/lib/vault";

// entry present -> edit mode (password field optional, blank keeps the
// current one); absent -> create mode (password required).
export function VaultEntryForm({
  entry,
  onDone,
}: {
  entry?: VaultEntry;
  onDone?: () => void;
}) {
  const action = entry ? updateVaultEntryAction : createVaultEntryAction;
  const [state, formAction, pending] = useActionState<VaultFormState, FormData>(action, vaultFormInitialState);

  useEffect(() => {
    if (state.status === "success") onDone?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state]);

  return (
    <form action={formAction} className="form-grid">
      {entry && <input type="hidden" name="id" value={entry.id} />}

      <div className="field">
        <label htmlFor="v-title">Título</label>
        <input id="v-title" name="title" type="text" placeholder="Ex: Netflix" defaultValue={entry?.title} required />
      </div>

      <div className="row2">
        <div className="field">
          <label htmlFor="v-user">Usuário</label>
          <input id="v-user" name="username" type="text" placeholder="usuario@email.com" defaultValue={entry?.username} />
        </div>
        <div className="field">
          <label htmlFor="v-pass">{entry ? "Nova senha (opcional)" : "Senha"}</label>
          <input
            id="v-pass"
            name="password"
            type="password"
            placeholder={entry ? "Deixe em branco para manter" : "Senha"}
            required={!entry}
            autoComplete="new-password"
          />
        </div>
      </div>

      <div className="field">
        <label htmlFor="v-url">URL</label>
        <input id="v-url" name="url" type="text" placeholder="https://" defaultValue={entry?.url} />
      </div>

      <div className="field">
        <label htmlFor="v-notes">Notas</label>
        <textarea id="v-notes" name="notes" rows={2} defaultValue={entry?.notes} />
      </div>

      <button className="btn-block" type="submit" disabled={pending}>
        {pending ? "Salvando…" : entry ? "Salvar alterações" : "Adicionar senha"}
      </button>

      {state.status === "error" && <p className="form-error">{state.message}</p>}
      {state.status === "success" && <p className="form-success">{state.message}</p>}
    </form>
  );
}
