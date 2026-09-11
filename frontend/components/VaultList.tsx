"use client";

import { useEffect, useRef, useState } from "react";
import { deleteVaultEntryAction } from "@/app/senhas/actions";
import { VaultEntryForm } from "@/components/VaultEntryForm";
import type { VaultEntry } from "@/lib/vault";
import { assertionCredentialToJSON, requestOptionsFromServer } from "@/lib/webauthn-encoding";

const REVEAL_SECONDS = 10;

type RevealState = { password: string; secondsLeft: number };

export function VaultList({ entries, canReveal }: { entries: VaultEntry[]; canReveal: boolean }) {
  const [revealed, setRevealed] = useState<Record<string, RevealState>>({});
  const [revealing, setRevealing] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [editingId, setEditingId] = useState<string | null>(null);
  const timers = useRef<Record<string, ReturnType<typeof setInterval>>>({});

  useEffect(() => {
    const activeTimers = timers.current;
    return () => {
      Object.values(activeTimers).forEach(clearInterval);
    };
  }, []);

  function startCountdown(id: string) {
    clearInterval(timers.current[id]);
    timers.current[id] = setInterval(() => {
      setRevealed((prev) => {
        const current = prev[id];
        if (!current) return prev;
        if (current.secondsLeft <= 1) {
          const next = { ...prev };
          delete next[id];
          clearInterval(timers.current[id]);
          return next;
        }
        return { ...prev, [id]: { ...current, secondsLeft: current.secondsLeft - 1 } };
      });
    }, 1000);
  }

  async function revelar(id: string) {
    setErrors((e) => ({ ...e, [id]: "" }));
    setRevealing(id);
    try {
      const beginRes = await fetch(`/api/vault-webauthn/reveal/${id}/begin`, { method: "POST" });
      if (!beginRes.ok) {
        const body = await beginRes.json().catch(() => null);
        throw new Error(body?.error ?? "Configure o Touch ID antes de revelar uma senha.");
      }
      const begin = await beginRes.json();

      const credential = (await navigator.credentials.get(
        requestOptionsFromServer(begin.options.publicKey),
      )) as PublicKeyCredential | null;
      if (!credential) throw new Error("Nenhuma confirmação recebida.");

      const finishRes = await fetch(
        `/api/vault-webauthn/reveal/${id}/finish?session=${encodeURIComponent(begin.reveal_session)}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(assertionCredentialToJSON(credential)),
        },
      );
      if (!finishRes.ok) throw new Error("Não foi possível confirmar sua identidade.");
      const { password } = await finishRes.json();

      setRevealed((prev) => ({ ...prev, [id]: { password, secondsLeft: REVEAL_SECONDS } }));
      startCountdown(id);
    } catch (err) {
      setErrors((e) => ({ ...e, [id]: friendlyMessage(err) }));
    } finally {
      setRevealing(null);
    }
  }

  async function copiar(password: string) {
    try {
      await navigator.clipboard.writeText(password);
    } catch {
      // Clipboard access can be denied by the browser; the password is
      // still visible on screen so the user can copy it by hand.
    }
  }

  if (entries.length === 0) {
    return <p className="empty-note">Nenhuma senha cadastrada ainda.</p>;
  }

  return (
    <div className="vault-list">
      {entries.map((entry) => {
        const state = revealed[entry.id];
        return (
          <div className="vault-row" key={entry.id}>
            {editingId === entry.id ? (
              <div className="vault-row-edit">
                <VaultEntryForm entry={entry} onDone={() => setEditingId(null)} />
                <button className="btn-text" type="button" onClick={() => setEditingId(null)}>
                  Cancelar
                </button>
              </div>
            ) : (
              <>
                <div className="vault-row-main">
                  <div className="vault-row-title">{entry.title}</div>
                  <div className="vault-row-meta">{entry.username || "sem usuário"}</div>
                </div>

                <div className="vault-row-password">
                  {state ? (
                    <span className="vault-password-reveal">
                      <code>{state.password}</code>
                      <span className="vault-countdown">{state.secondsLeft}s</span>
                    </span>
                  ) : (
                    <span className="vault-password-mask">••••••••••</span>
                  )}
                </div>

                <div className="vault-row-actions">
                  {state ? (
                    <button className="btn-text" type="button" onClick={() => copiar(state.password)}>
                      Copiar
                    </button>
                  ) : canReveal ? (
                    <button
                      className="btn-text"
                      type="button"
                      onClick={() => revelar(entry.id)}
                      disabled={revealing === entry.id}
                    >
                      {revealing === entry.id ? "Confirmando…" : "Revelar"}
                    </button>
                  ) : (
                    <span className="vault-row-meta">configure o Touch ID para revelar</span>
                  )}
                  <button className="btn-text" type="button" onClick={() => setEditingId(entry.id)}>
                    Editar
                  </button>
                  <form action={deleteVaultEntryAction.bind(null, entry.id)}>
                    <button className="btn-text bad" type="submit">
                      Excluir
                    </button>
                  </form>
                </div>

                {errors[entry.id] && <p className="form-error vault-row-error">{errors[entry.id]}</p>}
              </>
            )}
          </div>
        );
      })}
    </div>
  );
}

function friendlyMessage(err: unknown): string {
  if (err instanceof DOMException && err.name === "NotAllowedError") {
    return "Confirmação cancelada.";
  }
  if (err instanceof Error) return err.message;
  return "Não foi possível revelar a senha.";
}
