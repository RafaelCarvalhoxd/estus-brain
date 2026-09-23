"use client";

import { useEffect, useState } from "react";
import type { AssistantSettings } from "./types";
import { IconClose } from "../icons";

// Where the owner points the chat at an external agent, and finds what to
// paste into that agent's own MCP config to use Estus Brain from outside.

function Copy({ text, label = "Copiar" }: { text: string; label?: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className="btn-text"
      onClick={() => {
        void navigator.clipboard.writeText(text).then(() => {
          setDone(true);
          setTimeout(() => setDone(false), 1500);
        });
      }}
    >
      {done ? "Copiado" : label}
    </button>
  );
}

// Shows the snippet with the token hidden unless revealed; copying always
// copies the real thing.
function Snippet({ title, text, hint, token, reveal }: { title: string; text: string; hint?: string; token: string; reveal: boolean }) {
  const shown = reveal || !token ? text : text.split(token).join("••••••••");
  return (
    <div className="es-snippet">
      <div className="es-snippet-head">
        <b>{title}</b>
        <Copy text={text} />
      </div>
      {hint && <p className="es-hint">{hint}</p>}
      <pre>{shown}</pre>
    </div>
  );
}

export function EngineSettings({
  settings,
  onClose,
  onChanged,
}: {
  settings: AssistantSettings | null;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [tab, setTab] = useState<"agent" | "mcp">("agent");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [reveal, setReveal] = useState(false);
  const [origin, setOrigin] = useState("");

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOrigin(window.location.origin);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  // True when saved, so the form knows whether to clear the token it sent.
  const save = async (body: Record<string, unknown>): Promise<boolean> => {
    setSaving(true);
    setError(null);
    try {
      const res = await fetch("/api/assistant/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as { error?: string } | null;
        setError(data?.error ?? "Não foi possível salvar.");
        return false;
      }
      await onChanged();
      return true;
    } catch {
      setError("Não foi possível falar com o servidor do Estus. Tente de novo.");
      return false;
    } finally {
      setSaving(false);
    }
  };

  const token = settings?.mcp.token ?? "";
  const shown = reveal ? token : token ? `${token.slice(0, 10)}…${token.slice(-4)}` : "";
  const mcpURL = `${origin}/mcp`;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-shell is-wide es" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Agente e conexões">
        <div className="panel">
          <button className="icon-btn modal-close" type="button" aria-label="Fechar" onClick={onClose}>
            <IconClose />
          </button>
          <div className="panel-head">
            <h2>Agente e conexões</h2>
          </div>
          <div className="seg es-tabs" role="tablist">
            <button type="button" role="tab" aria-selected={tab === "agent"} className={tab === "agent" ? "active" : ""} onClick={() => setTab("agent")}>
              Agente
            </button>
            <button type="button" role="tab" aria-selected={tab === "mcp"} className={tab === "mcp" ? "active" : ""} onClick={() => setTab("mcp")}>
              MCP
            </button>
          </div>
          {error && <p className="form-error">{error}</p>}

          {tab === "agent" &&
            (settings === null ? <p className="es-hint">Verificando o agente…</p> : <AgentForm settings={settings} saving={saving} save={save} onTest={onChanged} />)}

          {tab === "mcp" && settings && (
            <div className="es-mcp">
              <p className="es-hint">
                O agente lê e grava no Estus por aqui: gastos, contas, notas, lembretes, agenda, hábitos… O cofre de senhas nunca é exposto. Configure
                este endereço e o token no MCP do seu agente.
              </p>
              <div className="es-token">
                <span>Token</span>
                <code>{shown}</code>
                <button type="button" className="btn-text" onClick={() => setReveal((v) => !v)}>
                  {reveal ? "Ocultar" : "Mostrar"}
                </button>
                <Copy text={token} />
              </div>
              <Snippet
                token={token}
                reveal={reveal}
                title="Hermes Agent"
                hint="Em ~/.hermes/config.yaml, na seção mcp_servers."
                text={`mcp_servers:\n  estus:\n    url: "${settings.mcp.url}"\n    headers:\n      Authorization: "Bearer ${token}"`}
              />
              <Snippet
                token={token}
                reveal={reveal}
                title="OpenClaw e outros"
                hint="Qualquer agente com MCP por HTTP: este endereço, com o token no cabeçalho Authorization."
                text={`URL: ${settings.mcp.url}\nAuthorization: Bearer ${token}`}
              />
              <p className="es-hint">Agente em outra máquina: troque o endereço por {mcpURL}.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function AgentForm({
  settings,
  saving,
  save,
  onTest,
}: {
  settings: AssistantSettings;
  saving: boolean;
  save: (body: Record<string, unknown>) => Promise<boolean>;
  onTest: () => Promise<void>;
}) {
  const { agent } = settings;
  const status = settings.providers.find((p) => p.id === "agent");
  // The inputs hold only what this screen saved; .env values show as
  // placeholders, so saving never copies them into the database.
  const [url, setURL] = useState(agent.url);
  const [model, setModel] = useState(agent.model);
  const [token, setToken] = useState("");
  const [testing, setTesting] = useState(false);
  const on = settings.provider === "agent";
  const dirty = url !== agent.url || model !== agent.model || token.trim() !== "";
  const fromEnv = (value: string) => `do .env: ${value}`;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const body: Record<string, unknown> = { agent_url: url, agent_model: model, provider: "agent" };
    if (token.trim()) body.agent_token = token;
    if (await save(body)) setToken("");
  };

  const test = async () => {
    setTesting(true);
    try {
      await onTest();
    } finally {
      setTesting(false);
    }
  };

  const tokenPlaceholder = agent.has_token
    ? "Token salvo — cole outro para trocar"
    : agent.env_token
      ? fromEnv("AGENT_TOKEN")
      : "API_SERVER_KEY do Hermes ou token do Gateway";

  return (
    <form className="es-agent" onSubmit={(e) => void submit(e)}>
      <p className={`es-hint${status?.available ? " is-ok" : ""}`}>
        <b>{status?.available ? "Conectado." : "Não conectado."}</b> {status?.detail}
      </p>
      <label className="es-field">
        <span>Endereço</span>
        <input value={url} placeholder={agent.env_url ? fromEnv(agent.env_url) : "http://127.0.0.1:8642"} onChange={(e) => setURL(e.target.value)} />
      </label>
      <label className="es-field">
        <span>Token</span>
        <input
          type="password"
          value={token}
          autoComplete="off"
          placeholder={tokenPlaceholder}
          onChange={(e) => setToken(e.target.value)}
          disabled={!settings.can_store_keys}
        />
      </label>
      {!settings.can_store_keys && <p className="es-hint">Para salvar o token aqui, defina VAULT_ENCRYPTION_KEY. Ou use AGENT_TOKEN no .env.</p>}
      <label className="es-field">
        <span>Modelo</span>
        {status?.models && status.models.length > 0 ? (
          <select value={model} onChange={(e) => setModel(e.target.value)}>
            <option value="">{agent.env_model ? fromEnv(agent.env_model) : "O primeiro que o agente oferece"}</option>
            {status.models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        ) : (
          <input value={model} placeholder={agent.env_model ? fromEnv(agent.env_model) : "hermes-agent ou openclaw/default"} onChange={(e) => setModel(e.target.value)} />
        )}
      </label>
      <div className="es-inline">
        <button type="submit" className="btn-primary" disabled={saving || (!dirty && on)}>
          {saving ? "Salvando…" : "Salvar e ligar"}
        </button>
        <button type="button" className="btn-outline" onClick={() => void test()} disabled={testing || saving}>
          {testing ? "Testando…" : "Testar conexão"}
        </button>
        {on && (
          <button type="button" className="btn-text" onClick={() => void save({ provider: "none" })}>
            Desligar
          </button>
        )}
        {agent.has_token && (
          <button type="button" className="btn-text" onClick={() => void save({ agent_token: "" })}>
            Remover token
          </button>
        )}
      </div>
      <p className="es-hint">Voz: o ditado e a leitura usam o próprio navegador. No Chrome, o ditado passa pelo servidor do Google.</p>
    </form>
  );
}
