"use client";

import { useEffect, useState } from "react";
import type { AssistantSettings, Capabilities, ProviderStatus } from "./types";
import { IconClose } from "../icons";

// Where the owner picks and configures the AI engines, and finds what to
// paste into Claude Desktop, Claude Code or Codex to use Estus Brain from
// outside (MCP).

const KIND_LABEL = { login: "Sua conta", api: "API key", local: "Local" } as const;

// Shows only what's missing — an engine with everything gets no line at all,
// so the list stays useful instead of repeating "✓ tudo" on every row.
function CapabilityGaps({ capabilities: c }: { capabilities: Capabilities }) {
  const gaps = [
    !c.supports_image_gen && "Não gera imagem",
    !c.supports_vision && "Não vê imagem",
    !c.supports_voice && "Sem chat de voz",
  ].filter(Boolean) as string[];
  if (gaps.length === 0) return null;
  return (
    <span className="es-engine-gaps">
      {gaps.map((g) => (
        <em key={g}>{g}</em>
      ))}
    </span>
  );
}

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

export function EngineSettings({ settings, onClose, onChanged }: { settings: AssistantSettings | null; onClose: () => void; onChanged: () => void }) {
  const [tab, setTab] = useState<"engines" | "mcp">("engines");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<string | null>(null);
  const [reveal, setReveal] = useState(false);
  const [origin, setOrigin] = useState("");

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOrigin(window.location.origin);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const save = async (key: string, body: Record<string, unknown>) => {
    setSaving(key);
    setError(null);
    const res = await fetch("/api/assistant/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    setSaving(null);
    if (!res.ok) {
      const data = (await res.json().catch(() => null)) as { error?: string } | null;
      setError(data?.error ?? "Não foi possível salvar.");
      return;
    }
    onChanged();
  };

  const token = settings?.mcp.token ?? "";
  const shown = reveal ? token : token ? `${token.slice(0, 10)}…${token.slice(-4)}` : "";
  const appURL = `${origin}/mcp`;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-shell is-wide es" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Motor de IA e conexões">
        <div className="panel">
          <button className="icon-btn modal-close" type="button" aria-label="Fechar" onClick={onClose}>
            <IconClose />
          </button>
          <div className="panel-head">
            <h2>Motor de IA e conexões</h2>
          </div>
          <div className="seg es-tabs" role="tablist">
            <button type="button" role="tab" aria-selected={tab === "engines"} className={tab === "engines" ? "active" : ""} onClick={() => setTab("engines")}>
              Motores no chat
            </button>
            <button type="button" role="tab" aria-selected={tab === "mcp"} className={tab === "mcp" ? "active" : ""} onClick={() => setTab("mcp")}>
              Usar por fora (MCP)
            </button>
          </div>
          {error && <p className="form-error">{error}</p>}

          {tab === "engines" &&
            (settings === null ? (
              <p className="es-hint">Verificando os motores…</p>
            ) : (
              <div className="es-engines">
                <label className={`es-engine${settings.provider === "none" ? " is-on" : ""}`}>
                  <input type="radio" name="engine" checked={settings.provider === "none"} onChange={() => void save("provider", { provider: "none" })} />
                  <div>
                    <b>Só atalhos</b>
                    <span>Respostas prontas, sem IA nenhuma.</span>
                  </div>
                </label>
                {settings.providers.map((p) => (
                  <EngineRow key={p.id} p={p} selected={settings.provider === p.id} saving={saving} settings={settings} save={save} />
                ))}
                {settings.voice && (
                  <p className={`es-hint${settings.voice.available ? " is-ok" : ""}`}>
                    <b>Voz:</b> {settings.voice.detail}
                  </p>
                )}
                {settings.multi_user && <p className="es-hint">Modo multiusuário: os motores que usam a sua assinatura pessoal ficam desligados.</p>}
                <p className="es-hint">
                  &quot;Sua conta&quot; usa o Claude Code ou o Codex já logados neste servidor, na sua assinatura. É para uso pessoal: se abrir o app
                  para outras pessoas, use API keys, Ollama ou Apple Intelligence (defina ASSISTANT_MULTI_USER=true).
                </p>
              </div>
            ))}

          {tab === "mcp" && settings && (
            <div className="es-mcp">
              <p className="es-hint">
                O Estus Brain expõe as mesmas ferramentas do chat (lançar gastos, contas, notas, lembretes, agenda, hábitos…) pelo protocolo MCP. Conecte
                seu Claude ou ChatGPT e peça as coisas de lá. O cofre de senhas nunca é exposto.
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
                title="Claude Code"
                hint="Neste servidor, ou em outra máquina trocando o endereço."
                text={`claude mcp add --transport http estus-brain ${settings.mcp.url} --header "Authorization: Bearer ${token}"`}
              />
              <Snippet
                token={token}
                reveal={reveal}
                title="Codex (GPT)"
                hint="Em ~/.codex/config.toml; exporte ESTUS_MCP_TOKEN com o token."
                text={`[mcp_servers.estus-brain]\nurl = "${settings.mcp.url}"\nbearer_token_env_var = "ESTUS_MCP_TOKEN"`}
              />
              <Snippet
                token={token}
                reveal={reveal}
                title="Claude Desktop"
                hint="Compile o relay com `go build -o estus-mcp ./cmd/estus-mcp` (em backend/) e aponte para o app."
                text={JSON.stringify(
                  {
                    mcpServers: {
                      "estus-brain": {
                        command: "/caminho/para/estus-mcp",
                        env: { ESTUS_MCP_URL: appURL, ESTUS_MCP_TOKEN: token },
                      },
                    },
                  },
                  null,
                  2,
                )}
              />
              <div className="es-snippet">
                <div className="es-snippet-head">
                  <b>Claude.ai e ChatGPT na web</b>
                </div>
                <p className="es-hint">
                  Os conectores web chamam o MCP a partir da nuvem deles, então o endereço precisa ser público. O /mcp não pede a senha do app, só o
                  token. Enquanto o login OAuth não existe, dá para usar o endereço com o token embutido; trate esse link como uma senha:
                </p>
                <pre>{`https://SEU-ENDERECO-PUBLICO/mcp?token=${reveal ? token : "…"}`}</pre>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function EngineRow({
  p,
  selected,
  saving,
  settings,
  save,
}: {
  p: ProviderStatus;
  selected: boolean;
  saving: string | null;
  settings: AssistantSettings;
  save: (key: string, body: Record<string, unknown>) => Promise<void>;
}) {
  const [model, setModel] = useState(p.model);
  const [key, setKey] = useState("");
  const [ollamaURL, setOllamaURL] = useState(settings.ollama_url);

  return (
    <div className={`es-engine${selected ? " is-on" : ""}${p.available ? "" : " is-off"}`}>
      <label className="es-engine-pick">
        <input type="radio" name="engine" checked={selected} disabled={!p.available} onChange={() => void save("provider", { provider: p.id })} />
        <div>
          <b>
            {p.name} <em>{KIND_LABEL[p.kind]}</em>
          </b>
          <span className={p.available ? "is-ok" : ""}>{p.detail}</span>
          <CapabilityGaps capabilities={p.capabilities} />
        </div>
      </label>
      <div className="es-engine-config">
        {p.id !== "apple" && (
          <div className="es-inline">
            {p.models && p.models.length > 0 ? (
              <select value={model} onChange={(e) => setModel(e.target.value)} aria-label="Modelo">
                {!p.models.includes(model) && model && <option value={model}>{model}</option>}
                {p.models.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            ) : (
              <input value={model} placeholder={p.id === "codex" ? "padrão da conta" : "modelo"} onChange={(e) => setModel(e.target.value)} aria-label="Modelo" />
            )}
            {model !== p.model && (
              <button type="button" className="btn-outline" disabled={saving === `model-${p.id}`} onClick={() => void save(`model-${p.id}`, { models: { [p.id]: model } })}>
                Salvar modelo
              </button>
            )}
          </div>
        )}
        {p.needs_key && (
          <div className="es-inline">
            <input type="password" value={key} placeholder={p.has_key ? "Chave salva — cole outra para trocar" : "Cole a API key"} onChange={(e) => setKey(e.target.value)} aria-label="API key" autoComplete="off" />
            <button
              type="button"
              className="btn-outline"
              disabled={!key.trim() || saving === `key-${p.id}` || !settings.can_store_keys}
              onClick={() => void save(`key-${p.id}`, { keys: { [p.id]: key } }).then(() => setKey(""))}
            >
              Salvar chave
            </button>
            {p.has_key && (
              <button type="button" className="btn-text" onClick={() => void save(`key-${p.id}`, { keys: { [p.id]: "" } })}>
                Remover
              </button>
            )}
          </div>
        )}
        {p.id === "ollama" && (
          <div className="es-inline">
            <input value={ollamaURL} onChange={(e) => setOllamaURL(e.target.value)} aria-label="Endereço do Ollama" />
            {ollamaURL !== settings.ollama_url && (
              <button type="button" className="btn-outline" onClick={() => void save("ollama", { ollama_url: ollamaURL })}>
                Salvar endereço
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
