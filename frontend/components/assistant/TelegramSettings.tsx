"use client";

import { useCallback, useEffect, useState } from "react";
import type { TelegramView } from "./types";

// The Telegram tab: the bot token, pairing the owner's chat, and what the bot
// sends on its own. Everything goes through /api/assistant/telegram.

async function call<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const res = await fetch(`/api/assistant/telegram/${path}`, {
    method,
    cache: "no-store",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = (await res.json().catch(() => null)) as (T & { error?: string }) | null;
  if (!res.ok) throw new Error(data?.error ?? "Não foi possível salvar.");
  return data as T;
}

function Switch({ on, label, disabled, onChange }: { on: boolean; label: string; disabled?: boolean; onChange: (on: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      className={`switch${on ? " on" : ""}`}
      disabled={disabled}
      onClick={() => onChange(!on)}
    />
  );
}

export function TelegramSettings() {
  const [view, setView] = useState<TelegramView | null>(null);
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());

  const load = useCallback(async () => {
    try {
      setView(await call<TelegramView>("settings"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não foi possível carregar.");
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  // While a code is in force, watch for the owner sending it to the bot.
  const waiting = !!view?.pairing_code && !view.paired;
  useEffect(() => {
    if (!waiting) return;
    const poll = setInterval(() => void load(), 3000);
    const clock = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      clearInterval(poll);
      clearInterval(clock);
    };
  }, [waiting, load]);

  const run = async (key: string, action: () => Promise<unknown>, done?: string): Promise<boolean> => {
    setBusy(key);
    setError(null);
    setNotice(null);
    try {
      await action();
      if (done) setNotice(done);
      await load();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não deu certo.");
      return false;
    } finally {
      setBusy(null);
    }
  };

  const save = (key: string, body: Record<string, unknown>) => run(key, () => call("settings", "PUT", body));

  if (!view) return <p className="es-hint">{error ?? "Carregando…"}</p>;

  const secondsLeft = view.pairing_expires_at ? Math.max(0, Math.round((new Date(view.pairing_expires_at).getTime() - now) / 1000)) : 0;
  const countdown = `${Math.floor(secondsLeft / 60)}:${String(secondsLeft % 60).padStart(2, "0")}`;

  return (
    <div className="es-telegram">
      {error && <p className="form-error">{error}</p>}
      {notice && <p className="es-hint is-ok">{notice}</p>}
      <p className={`es-status is-${view.status.state}`}>{view.status.message}</p>

      <section className="es-snippet">
        <div className="es-snippet-head">
          <b>1. Token do bot</b>
        </div>
        <p className="es-hint">
          No Telegram, abra o{" "}
          <a href="https://t.me/BotFather" target="_blank" rel="noreferrer">
            @BotFather
          </a>
          , mande /newbot, escolha um nome e cole aqui o token que ele devolver.
        </p>
        {view.from_env && <p className="es-hint">Usando TELEGRAM_BOT_TOKEN do servidor{view.bot_username ? ` (@${view.bot_username})` : ""}.</p>}
        {/* A saved token wins over the server's, so it can still be swapped here. */}
        {(!view.from_env || view.can_store_token) && (
          <div className="es-inline">
            <input
              type="password"
              value={token}
              autoComplete="off"
              aria-label="Token do bot"
              placeholder={
                view.from_env
                  ? "Cole um token para usar outro bot"
                  : view.has_token
                    ? `Conectado a @${view.bot_username} — cole outro para trocar`
                    : "Cole o token do @BotFather"
              }
              disabled={!view.can_store_token || busy === "token"}
              onChange={(e) => setToken(e.target.value)}
            />
            <button
              type="button"
              className="btn-outline"
              disabled={!token.trim() || busy === "token"}
              onClick={() => void save("token", { token }).then((ok) => ok && setToken(""))}
            >
              {busy === "token" ? "Verificando…" : "Salvar token"}
            </button>
            {view.has_token && !view.from_env && (
              <button type="button" className="btn-text" disabled={busy === "token"} onClick={() => void save("token", { token: "" })}>
                Remover
              </button>
            )}
          </div>
        )}
        {!view.can_store_token && !view.from_env && (
          <p className="es-hint">Defina VAULT_ENCRYPTION_KEY para salvar o token, ou use TELEGRAM_BOT_TOKEN no servidor.</p>
        )}
      </section>

      {view.has_token && (
        <section className="es-snippet">
          <div className="es-snippet-head">
            <b>2. Parear com o seu Telegram</b>
          </div>
          {view.paired ? (
            <div className="es-inline es-row">
              <span>
                Pareado com <b>{view.owner_name || "você"}</b>. Só esse chat é atendido.
              </span>
              <button
                type="button"
                className="btn-text"
                disabled={busy === "pair"}
                onClick={() => {
                  if (window.confirm("Desparear o Telegram? O bot para de responder e de mandar avisos.")) void run("pair", () => call("pair", "DELETE"));
                }}
              >
                Desparear
              </button>
            </div>
          ) : view.pairing_code && secondsLeft > 0 ? (
            <>
              <p className="es-hint">Mande esta mensagem para o bot em até {countdown}:</p>
              <div className="es-token">
                <code>/start {view.pairing_code}</code>
                {view.bot_username && (
                  <a className="btn-outline" href={`https://t.me/${view.bot_username}?start=${view.pairing_code}`} target="_blank" rel="noreferrer">
                    Abrir no Telegram
                  </a>
                )}
              </div>
            </>
          ) : (
            <button type="button" className="btn-outline" disabled={busy === "pair"} onClick={() => void run("pair", () => call("pair", "POST"))}>
              Gerar código
            </button>
          )}
        </section>
      )}

      {view.paired && (
        <section className="es-snippet">
          <div className="es-snippet-head">
            <b>3. Avisos</b>
            <button type="button" className="btn-text" disabled={busy === "test"} onClick={() => void run("test", () => call("test", "POST"), "Mensagem de teste enviada.")}>
              Mandar teste
            </button>
          </div>
          <div className="es-notice">
            <Switch on={view.morning_enabled} label="Resumo da manhã" disabled={!!busy} onChange={(on) => void save("morning", { morning_enabled: on })} />
            <span>Resumo da manhã</span>
            <input
              key={view.morning_time}
              type="time"
              min="04:00"
              max="11:59"
              defaultValue={view.morning_time}
              disabled={!!busy}
              aria-label="Horário do resumo da manhã"
              onBlur={(e) => {
                if (e.target.value && e.target.value !== view.morning_time) void save("morning", { morning_time: e.target.value });
              }}
            />
          </div>
          <div className="es-notice">
            <Switch on={view.evening_enabled} label="Fechamento da noite" disabled={!!busy} onChange={(on) => void save("evening", { evening_enabled: on })} />
            <span>Fechamento da noite</span>
            <input
              key={view.evening_time}
              type="time"
              min="12:00"
              max="23:59"
              defaultValue={view.evening_time}
              disabled={!!busy}
              aria-label="Horário do fechamento da noite"
              onBlur={(e) => {
                if (e.target.value && e.target.value !== view.evening_time) void save("evening", { evening_time: e.target.value });
              }}
            />
          </div>
          <div className="es-notice">
            <Switch on={view.reminders_enabled} label="Lembrete na hora" disabled={!!busy} onChange={(on) => void save("reminders", { reminders_enabled: on })} />
            <span>Lembrete na hora, com botão “Concluído”</span>
            <span />
          </div>
          <div className="es-notice">
            <Switch on={view.events_enabled} label="Aviso de compromisso" disabled={!!busy} onChange={(on) => void save("events", { events_enabled: on })} />
            <span>Aviso antes de compromisso</span>
            <span className="es-notice-minutes">
              <input
                key={view.events_minutes_before}
                type="number"
                min={1}
                max={1440}
                defaultValue={view.events_minutes_before}
                disabled={!!busy}
                aria-label="Minutos antes do compromisso"
                onBlur={(e) => {
                  const minutes = Number(e.target.value);
                  if (minutes && minutes !== view.events_minutes_before) void save("events", { events_minutes_before: minutes });
                }}
              />
              min antes
            </span>
          </div>
          <p className="es-hint">
            Com o computador dormindo nada sai; ao acordar, o resumo da manhã ainda vai até o meio-dia, o da noite até meia-noite e lembretes com até 12 h
            de atraso. No Telegram: /hoje, /noite, /nova e /ajuda.
          </p>
        </section>
      )}
    </div>
  );
}
