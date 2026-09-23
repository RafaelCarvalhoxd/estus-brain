"use client";

import { useEffect, useRef, useState } from "react";
import { deleteVaultEntryAction } from "@/app/senhas/actions";
import { Modal } from "@/components/Modal";
import { VaultEntryForm } from "@/components/VaultEntryForm";
import type { VaultEntry } from "@/lib/vault";
import { IconSearch } from "./icons";

const REVEAL_SECONDS = 10;

type RevealState = { password: string; secondsLeft: number };

export function VaultList({ entries }: { entries: VaultEntry[] }) {
  const [revealed, setRevealed] = useState<Record<string, RevealState>>({});
  const [revealing, setRevealing] = useState(false);
  const [askingFor, setAskingFor] = useState<VaultEntry | null>(null);
  const [askError, setAskError] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
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

  function pedirSenha(entry: VaultEntry) {
    setAskError("");
    setAskingFor(entry);
  }

  async function revelar(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!askingFor) return;
    const id = askingFor.id;
    const appPassword = String(new FormData(e.currentTarget).get("password") ?? "");
    setAskError("");
    setRevealing(true);
    try {
      const res = await fetch(`/api/vault/${id}/reveal`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password: appPassword }),
      });
      if (res.status === 401) throw new Error("Senha incorreta.");
      if (!res.ok) throw new Error("Não foi possível revelar a senha.");
      const { password } = await res.json();

      setRevealed((prev) => ({ ...prev, [id]: { password, secondsLeft: REVEAL_SECONDS } }));
      startCountdown(id);
      setAskingFor(null);
    } catch (err) {
      setAskError(err instanceof Error ? err.message : "Não foi possível revelar a senha.");
    } finally {
      setRevealing(false);
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

  const q = query.trim().toLowerCase();
  const visibleEntries = q
    ? entries.filter((e) => e.title.toLowerCase().includes(q) || e.username.toLowerCase().includes(q))
    : entries;

  return (
    <div className="vault-list">
      <div className="list-filters">
        <div className="filter-search">
          <IconSearch />
          <input type="text" placeholder="Buscar senhas" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
      </div>
      {visibleEntries.length === 0 && <p className="empty-note">Nenhuma senha bate com essa busca.</p>}
      {visibleEntries.map((entry) => {
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
                  ) : (
                    <button className="btn-text" type="button" onClick={() => pedirSenha(entry)}>
                      Revelar
                    </button>
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
              </>
            )}
          </div>
        );
      })}
      <Modal open={askingFor !== null} onClose={() => setAskingFor(null)}>
        <div className="panel">
          <div className="panel-head">
            <h2>Revelar {askingFor?.title}</h2>
          </div>
          <form onSubmit={revelar} className="form-grid">
            <div className="field">
              <label htmlFor="reveal-password">Senha do app</label>
              <input id="reveal-password" name="password" type="password" autoComplete="current-password" autoFocus required />
            </div>
            <button className="btn-block" type="submit" disabled={revealing}>
              {revealing ? "Revelando…" : "Revelar"}
            </button>
            {askError && <p className="form-error">{askError}</p>}
          </form>
        </div>
      </Modal>
    </div>
  );
}
