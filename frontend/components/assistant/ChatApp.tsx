"use client";
/* eslint-disable @typescript-eslint/no-explicit-any -- tool results are the backend's loose JSON, read field by field */

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type CSSProperties, type FormEvent } from "react";
import type { AssistantSettings, CardData, ChatItem, ConversationSummary, ModuleKey, ToolRun } from "./types";
import { FLOWS, MODULES, expensePrefill, flowById, matchFlows, type Flow, type FlowField, type OptionSource } from "./flows";
import { Markdown } from "./Markdown";
import { EngineSettings } from "./EngineSettings";
import { useVoiceRecorder } from "./useVoiceRecorder";
import { IconPlus, IconTrash } from "../icons";

// The chat: pick a module, tap a ready-made question or fill a short form,
// and the answer comes back as a sentence and a card — no AI needed. With an
// engine connected, anything typed goes to it instead, and it can use the
// very same tools. Which conversation is open lives in the URL (?c=).

let seq = 0;
const uid = () => `local-${Date.now()}-${seq++}`;

/** An assistant reply in the thread, and the words to read aloud for it. */
type Reply = { id: string; spoken: string };

const TOOL_LABELS: Record<string, string> = {
  finance_list_categories: "Consultou categorias",
  finance_list_credit_cards: "Consultou cartões",
  finance_create_transaction: "Lançou um gasto",
  finance_create_transactions: "Lançou gastos",
  finance_month_summary: "Consultou gastos do mês",
  finance_list_transactions: "Consultou lançamentos",
  finance_update_transaction: "Editou um lançamento",
  finance_delete_transaction: "Excluiu um lançamento",
  bills_list: "Consultou contas",
  bills_summary: "Consultou resumo das contas",
  bills_create: "Cadastrou uma conta",
  bills_mark_paid: "Marcou conta como paga",
  bills_delete: "Excluiu uma conta",
  notes_search: "Buscou notas",
  notes_read: "Leu uma nota",
  notes_create: "Criou uma nota",
  notes_append: "Completou uma nota",
  notes_delete: "Excluiu uma nota",
  reminders_list: "Consultou lembretes",
  reminders_create: "Criou um lembrete",
  reminders_set_done: "Concluiu um lembrete",
  reminders_delete: "Excluiu um lembrete",
  agenda_list: "Consultou a agenda",
  agenda_create: "Marcou um compromisso",
  agenda_delete: "Excluiu um compromisso",
  habits_today: "Consultou hábitos",
  habits_log: "Marcou um hábito",
  habits_create: "Criou um hábito",
  training_day: "Consultou o treino",
  diet_day: "Consultou a dieta",
  documents_search: "Buscou documentos",
  boards_list: "Consultou quadros",
  overview_today: "Montou o resumo do dia",
  generate_image: "Gerou uma imagem",
  edit_image: "Editou uma imagem",
  documents_read: "Leu um documento",
  documents_write: "Criou um arquivo",
};

/** Turns a finished generate_image call into the card the bubble renders —
 * null for every other tool, or if the result doesn't look like one
 * (an error, or a shape from before this ran). */
function imageCardFrom(e: { tool?: string; result?: unknown }): CardData | null {
  if ((e.tool !== "generate_image" && e.tool !== "edit_image") || !e.result || typeof e.result !== "object") return null;
  const r = e.result as { document_id?: unknown; descricao?: unknown };
  if (typeof r.document_id !== "string") return null;
  return { kind: "image", url: `/api/documents/${r.document_id}/download`, alt: typeof r.descricao === "string" ? r.descricao : "Imagem gerada" };
}

async function callTool(name: string, args: Record<string, unknown>): Promise<any> {
  const res = await fetch(`/api/assistant/tools/${name}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(args),
  });
  const data = (await res.json().catch(() => null)) as { result?: unknown; error?: string } | null;
  if (!res.ok) throw new Error(data?.error ?? "Não foi possível executar.");
  return data?.result;
}

async function loadOptions(source: OptionSource): Promise<{ value: string; label: string }[]> {
  const day = (d: string) => d.slice(0, 10).split("-").reverse().join("/");
  switch (source) {
    case "categories":
      return ((await callTool("finance_list_categories", {})).categorias ?? []).map((c: any) => ({ value: c.nome, label: c.nome }));
    case "cards":
      return ((await callTool("finance_list_credit_cards", {})).cartoes ?? []).map((c: any) => ({ value: c.nome, label: c.nome }));
    case "openBills":
      return ((await callTool("bills_list", { status: "abertas" })).contas ?? []).map((b: any) => ({ value: b.id, label: `${b.descricao} · ${b.valor} · ${day(b.vencimento)}` }));
    case "pendingReminders":
      return ((await callTool("reminders_list", { filter: "pendentes" })).lembretes ?? []).map((r: any) => ({ value: r.id, label: r.titulo }));
    case "upcomingEvents":
      return ((await callTool("agenda_list", { days: 30 })).compromissos ?? []).map((e: any) => ({ value: e.id, label: `${e.titulo} · ${day(e.inicio)}` }));
    case "todayHabits":
      return ((await callTool("habits_today", {})).habitos ?? []).map((h: any) => ({ value: h.habito, label: h.habito }));
  }
}

function itemsFromStored(messages: { id: string; role: "user" | "assistant"; content: string; data: any; provider: string }[]): ChatItem[] {
  return messages.map((m) => {
    const tools = ((m.data?.tool_calls as any[]) ?? []).map((t) => ({ id: t.id, name: t.name, args: t.args, result: t.result, error: t.error, done: true }));
    // generate_image/edit_image's card isn't stored separately (chat.go only
    // saves tool_calls) — rebuild it the same way the live stream does, so
    // reloading history doesn't lose the picture.
    let imageCard: CardData | undefined;
    for (const t of tools) {
      const card = imageCardFrom({ tool: t.name, result: t.result });
      if (card) {
        imageCard = card;
        break;
      }
    }
    return {
      id: m.id,
      role: m.role,
      text: m.content,
      card: (m.data?.card as CardData | undefined) ?? imageCard,
      tools,
      attachment: m.data?.attachment ? { name: m.data.attachment.name, content_type: m.data.attachment.content_type } : undefined,
      error: m.data?.error || undefined,
      errorDetail: m.data?.error_detail || undefined,
      provider: m.provider,
    };
  });
}

export function ChatApp({
  initialConversations,
  initialConversation,
}: {
  initialConversations: ConversationSummary[];
  initialConversation: { conversation: ConversationSummary; messages: any[] } | null;
}) {
  const [conversations, setConversations] = useState(initialConversations);
  const [conversationId, setConversationId] = useState<string | null>(initialConversation?.conversation.id ?? null);
  const [items, setItems] = useState<ChatItem[]>(() => (initialConversation ? itemsFromStored(initialConversation.messages) : []));
  const [module, setModule] = useState<ModuleKey>((initialConversation?.conversation.module as ModuleKey) || "geral");
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [settings, setSettings] = useState<AssistantSettings | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [engineMenu, setEngineMenu] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const threadRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [attachment, setAttachment] = useState<{ id: string; name: string; content_type: string } | null>(null);
  const [uploading, setUploading] = useState(false);
  const [attachError, setAttachError] = useState<string | null>(null);

  const loadSettings = useCallback(async () => {
    const res = await fetch("/api/assistant/settings", { cache: "no-store" });
    if (res.ok) setSettings((await res.json()) as AssistantSettings);
  }, []);

  useEffect(() => {
    // Checking each engine takes a moment, so it happens after first paint.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadSettings();
  }, [loadSettings]);

  const refreshConversations = useCallback(async () => {
    const res = await fetch("/api/assistant/conversations", { cache: "no-store" });
    if (res.ok) setConversations((await res.json()) as ConversationSummary[]);
  }, []);

  // Keep the newest message in view as the thread grows.
  useEffect(() => {
    const el = threadRef.current;
    if (el && items.length > 0) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  }, [items]);

  const engine = settings?.providers.find((p) => p.id === settings.provider && p.available) ?? null;
  const moduleColor = MODULES.find((m) => m.key === module)?.color ?? "var(--brain-glow)";

  const update = (id: string, patch: Partial<ChatItem> | ((item: ChatItem) => Partial<ChatItem>)) =>
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...(typeof patch === "function" ? patch(it) : patch) } : it)));

  const adoptConversation = (id: string) => {
    setConversationId(id);
    const url = new URL(window.location.href);
    url.searchParams.set("c", id);
    window.history.replaceState(null, "", url);
  };

  const record = async (user: string, assistant: string, card?: CardData) => {
    const res = await fetch("/api/assistant/record", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ conversation_id: conversationIdRef.current ?? "", module: module === "geral" ? "" : module, user, assistant, data: card ? { card } : {} }),
    });
    if (res.ok) {
      const { conversation_id } = (await res.json()) as { conversation_id: string };
      conversationIdRef.current = conversation_id;
      adoptConversation(conversation_id);
      void refreshConversations();
    }
  };
  // The id changes inside async flows; read the latest one there.
  const conversationIdRef = useRef(conversationId);
  useEffect(() => {
    conversationIdRef.current = conversationId;
  }, [conversationId]);

  // ---- flows

  const runFlow = async (flow: Flow, values: Record<string, string>, rows: Record<string, string>[], userText: string): Promise<Reply> => {
    const pendingId = uid();
    setItems((prev) => [...prev, { id: pendingId, role: "assistant", text: "", tools: [], pending: true }]);
    setBusy(true);
    try {
      const { tool, args } = flow.run(values, rows);
      const result = await callTool(tool, args);
      const { text, card } = flow.present(result, values);
      const next = FLOWS.filter((f) => f.module === flow.module && f.id !== flow.id).slice(0, 3).map((f) => f.id);
      update(pendingId, { text, card, pending: false, suggestions: next, provider: "fluxo" });
      await record(userText, text, card);
      return { id: pendingId, spoken: text };
    } catch (err) {
      const error = err instanceof Error ? err.message : "Não deu certo.";
      update(pendingId, { pending: false, error });
      return { id: pendingId, spoken: error };
    } finally {
      setBusy(false);
    }
  };

  const startFlow = async (flow: Flow, prefill?: Record<string, string>, userText?: string, voice = false): Promise<Reply> => {
    if (flow.module !== "geral" && module !== flow.module) setModule(flow.module);
    const ask = userText ?? flow.ask;
    setItems((prev) => [...prev, { id: uid(), role: "user", text: ask, tools: [], voice }]);
    if (!flow.fields && !flow.rowFields) return runFlow(flow, {}, [], ask);
    const formId = uid();
    const intro = flow.intro ?? "Preencha:";
    setItems((prev) => [...prev, { id: formId, role: "assistant", text: intro, tools: [], formFlow: flow.id, formPrefill: prefill }]);
    return { id: formId, spoken: intro };
  };

  const submitForm = (item: ChatItem, values: Record<string, string>, rows: Record<string, string>[]) => {
    const flow = flowById(item.formFlow!);
    if (!flow) return;
    update(item.id, { formValues: values, formRows: rows });
    const userText = items.find((it, i) => items[i + 1]?.id === item.id)?.text ?? flow.ask;
    void runFlow(flow, values, rows, userText);
  };

  // ---- attachments

  const onFileChosen = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ""; // lets picking the same file twice fire onChange again
    if (!file) return;
    setAttachError(null);
    setUploading(true);
    try {
      const form = new FormData();
      form.append("file", file, file.name);
      const res = await fetch("/api/assistant/attachments", { method: "POST", body: form });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        setAttachError(body?.error ?? "Não consegui anexar o arquivo.");
        return;
      }
      const doc = (await res.json()) as { document_id: string; name: string; content_type: string };
      setAttachment({ id: doc.document_id, name: doc.name, content_type: doc.content_type });
    } catch {
      setAttachError("Não consegui enviar o arquivo. Confira sua conexão.");
    } finally {
      setUploading(false);
    }
  };

  // ---- AI

  const sendToAI = async (text: string, voice = false): Promise<Reply> => {
    const assistantId = uid();
    const sentAttachment = attachment;
    setAttachment(null);
    setItems((prev) => [
      ...prev,
      { id: uid(), role: "user", text, tools: [], voice, attachment: sentAttachment ?? undefined },
      { id: assistantId, role: "assistant", text: "", tools: [], pending: true, provider: settings?.provider },
    ]);
    setBusy(true);
    const controller = new AbortController();
    abortRef.current = controller;
    setStreaming(true);
    let answer = "";
    let failure = "";
    let aborted = false;
    const fail = (error: string, errorDetail?: string) => {
      failure = error;
      update(assistantId, { error, errorDetail });
    };
    try {
      const res = await fetch("/api/assistant/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          conversation_id: conversationIdRef.current ?? "",
          message: text,
          module: module === "geral" ? "" : module,
          attachment_id: sentAttachment?.id ?? "",
        }),
        signal: controller.signal,
      });
      if (!res.ok || !res.body) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        fail(
          res.status >= 500
            ? "Não consegui falar com o servidor do Estus: o backend parece estar fora do ar. Tente de novo em instantes."
            : "O servidor recusou a mensagem. Tente de novo; se continuar, veja os detalhes técnicos.",
          body?.error ?? `HTTP ${res.status}`,
        );
      } else {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let finished = false;
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          let cut: number;
          while ((cut = buffer.indexOf("\n\n")) >= 0) {
            const chunk = buffer.slice(0, cut);
            buffer = buffer.slice(cut + 2);
            if (!chunk.startsWith("data: ")) continue;
            const e = JSON.parse(chunk.slice(6)) as { type: string; text?: string; conversation_id?: string; tool_id?: string; tool?: string; args?: unknown; result?: unknown; error?: string; detail?: string };
            if (e.type === "conversation" && e.conversation_id) {
              conversationIdRef.current = e.conversation_id;
              adoptConversation(e.conversation_id);
            } else if (e.type === "text") {
              answer += e.text ?? "";
              update(assistantId, (it) => ({ text: it.text + (e.text ?? "") }));
            } else if (e.type === "tool") {
              update(assistantId, (it) => ({ tools: [...it.tools, { id: e.tool_id ?? uid(), name: e.tool ?? "", args: e.args, done: false }] }));
            } else if (e.type === "tool_result") {
              update(assistantId, (it) => ({
                tools: it.tools.map((t) => (t.id === e.tool_id ? { ...t, result: e.result, error: e.error, done: true } : t)),
                card: imageCardFrom(e) ?? it.card,
              }));
            } else if (e.type === "error") {
              fail(e.error ?? "Não deu certo.", e.detail);
            } else if (e.type === "done") {
              finished = true;
            }
          }
        }
        if (!finished && !failure) {
          fail("A resposta foi interrompida antes de terminar: a conexão com o servidor caiu. Tente de novo.");
        }
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        aborted = true;
      } else {
        fail("Não consegui falar com o servidor do Estus. Confira se o app está rodando e tente de novo.", err instanceof Error ? err.message : String(err));
      }
    } finally {
      update(assistantId, (it) => ({ pending: false, tools: it.tools.map((t) => ({ ...t, done: true })) }));
      setBusy(false);
      setStreaming(false);
      abortRef.current = null;
      void refreshConversations();
    }
    return { id: assistantId, spoken: aborted ? "" : [answer.trim(), failure].filter(Boolean).join("\n\n") };
  };

  // ---- typing

  // Typed or spoken, a message goes the same way: to the AI engine, or to the
  // ready-made flows without one. The reply comes back for reading aloud.
  const sendText = async (text: string, voice: boolean): Promise<Reply> => {
    if (engine) return sendToAI(text, voice);
    const matches = matchFlows(text, module);
    const best = matches[0];
    // A strong match runs; a weaker one still opens a form, which the person can fix.
    if (best && (best.score >= 1.5 || (best.score >= 1 && (best.flow.fields || best.flow.rowFields)))) {
      const prefill = best.flow.id === "txn-create" ? expensePrefill(text) : undefined;
      return startFlow(best.flow, prefill, text, voice);
    }
    const suggestions = (matches.length ? matches.map((m) => m.flow) : FLOWS.filter((f) => f.popular && (module === "geral" || f.module === module)))
      .slice(0, 4)
      .map((f) => f.id);
    const replyId = uid();
    const reply = "Sem um motor de IA conectado, eu entendo os atalhos prontos. Talvez seja um destes, ou conecte uma IA para conversar livremente.";
    setItems((prev) => [
      ...prev,
      { id: uid(), role: "user", text, tools: [], voice },
      { id: replyId, role: "assistant", text: reply, tools: [], suggestions, provider: "fluxo" },
    ]);
    return { id: replyId, spoken: reply };
  };

  const submit = (e?: FormEvent) => {
    e?.preventDefault();
    const text = input.trim();
    // A lone attachment is a valid message when there's an engine to read
    // it — the flows path (no engine) has nothing to do with a bare file.
    if ((!text && !(engine && attachment)) || busy) return;
    setInput("");
    setVoiceNotice(null);
    stopSpeaking();
    void sendText(text || "Veja o anexo.", false);
  };

  // ---- voice

  const voiceOn = !!settings?.voice?.available;
  const playerRef = useRef<HTMLAudioElement | null>(null);
  // The Blob URL backing the current (or just-finished) player; revoked on
  // every exit path so a stopped or replaced answer doesn't leak its audio.
  const playerUrlRef = useRef<string | null>(null);
  const [speaking, setSpeaking] = useState(false);
  const [voiceNotice, setVoiceNotice] = useState<string | null>(null);

  const stopSpeaking = useCallback(() => {
    playerRef.current?.pause();
    playerRef.current = null;
    if (playerUrlRef.current) {
      URL.revokeObjectURL(playerUrlRef.current);
      playerUrlRef.current = null;
    }
    setSpeaking(false);
  }, []);

  // Leaving the chat shouldn't leave an answer talking to an empty room.
  useEffect(() => () => stopSpeaking(), [stopSpeaking]);

  const speak = async ({ id, spoken }: Reply) => {
    stopSpeaking();
    if (!spoken.trim()) return;
    let player: HTMLAudioElement | null = null;
    try {
      const res = await fetch("/api/assistant/voice/speak", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ text: spoken }) });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const url = URL.createObjectURL(await res.blob());
      playerUrlRef.current = url;
      player = new Audio(url);
      playerRef.current = player;
      const finish = () => {
        if (playerUrlRef.current === url) {
          URL.revokeObjectURL(url);
          playerUrlRef.current = null;
        }
        if (playerRef.current === player) {
          playerRef.current = null;
          setSpeaking(false);
        }
      };
      player.onended = finish;
      player.onerror = finish;
      setSpeaking(true);
      await player.play();
    } catch {
      // Also reached when player.play() rejects (e.g. autoplay policy): the
      // URL was already stashed above, so stopSpeaking() revokes it here too.
      // Stopping the playback by hand rejects play() the same way; whoever
      // stopped it already cleaned up, and the answer isn't unreadable.
      if (playerRef.current !== player) return;
      stopSpeaking();
      update(id, { spokenError: "Não consegui ler em voz alta." });
    }
  };

  const sendRecording = async (wav: Blob) => {
    setVoiceNotice(null);
    const res = await fetch("/api/assistant/voice/transcribe", {
      method: "POST",
      headers: { "Content-Type": "audio/wav", "X-Filename": "gravacao.wav" },
      body: wav,
    }).catch(() => null);
    const data = (await res?.json().catch(() => null)) as { text?: string; error?: string } | null;
    if (!res?.ok) {
      setItems((prev) => [...prev, { id: uid(), role: "assistant", text: "", tools: [], error: data?.error ?? "Não consegui transcrever o áudio. Tente de novo.", provider: "voz" }]);
      return;
    }
    const heard = data?.text?.trim() ?? "";
    if (!heard) {
      setVoiceNotice("Não ouvi nada — tente de novo.");
      return;
    }
    // Don't wait for the answer: the recorder is still showing "Transcrevendo…"
    // until this resolves, and the typing bubble already covers the answering.
    void sendText(heard, true).then((reply) => void speak(reply));
  };

  const recorder = useVoiceRecorder(sendRecording, setVoiceNotice);

  const newConversation = () => {
    abortRef.current?.abort();
    stopSpeaking();
    setItems([]);
    setConversationId(null);
    conversationIdRef.current = null;
    // The chat lives both at /chat and over the núcleo; stay where we are.
    window.history.replaceState(null, "", window.location.pathname);
    inputRef.current?.focus();
  };

  const openConversation = async (id: string) => {
    if (busy) return;
    const res = await fetch(`/api/assistant/conversations/${id}`, { cache: "no-store" });
    if (!res.ok) return;
    const data = (await res.json()) as { conversation: ConversationSummary; messages: any[] };
    setItems(itemsFromStored(data.messages));
    setModule((data.conversation.module as ModuleKey) || "geral");
    conversationIdRef.current = id;
    adoptConversation(id);
  };

  const removeConversation = async (id: string) => {
    if (!window.confirm("Apagar esta conversa?")) return;
    await fetch(`/api/assistant/conversations/${id}`, { method: "DELETE" });
    if (id === conversationId) newConversation();
    void refreshConversations();
  };

  const chooseEngine = async (provider: string) => {
    setEngineMenu(false);
    await fetch("/api/assistant/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ provider }) });
    void loadSettings();
  };

  const quick = useMemo(() => FLOWS.filter((f) => (module === "geral" ? f.popular : f.module === module)), [module]);

  return (
    <div className="cx" style={{ "--mod": moduleColor } as CSSProperties}>
      <aside className="panel cx-side" aria-label="Conversas">
        <button type="button" className="btn-primary cx-new" onClick={newConversation}>
          <IconPlus />
          Nova conversa
        </button>
        <div className="cx-side-head">Conversas</div>
        <ul className="cx-convs">
          {conversations.length === 0 && <li className="cx-muted">As conversas aparecem aqui.</li>}
          {conversations.map((c) => (
            <li key={c.id} className={c.id === conversationId ? "is-open" : ""}>
              <button type="button" className="cx-conv" onClick={() => void openConversation(c.id)}>
                <span>{c.title || "Conversa"}</span>
                <small>{new Date(c.updated_at).toLocaleDateString("pt-BR", { day: "numeric", month: "short" })}</small>
              </button>
              <button type="button" className="icon-btn bad" aria-label="Apagar conversa" onClick={() => void removeConversation(c.id)}>
                <IconTrash />
              </button>
            </li>
          ))}
        </ul>
        <button type="button" className="btn-outline cx-engine-settings" onClick={() => setSettingsOpen(true)}>
          Motor de IA e conexões
        </button>
      </aside>

      <section className="panel cx-main" aria-label="Conversa">
        <nav className="cx-modules" aria-label="Módulo">
          {MODULES.map((m) => (
            <button
              key={m.key}
              type="button"
              className={`cx-module${module === m.key ? " is-on" : ""}`}
              style={{ "--chip": m.color } as CSSProperties}
              aria-pressed={module === m.key}
              onClick={() => setModule(m.key)}
            >
              {m.label}
            </button>
          ))}
        </nav>

        <div className="cx-thread" ref={threadRef}>
          {items.length === 0 ? (
            <div className="cx-welcome">
              <h2>O que vamos fazer?</h2>
              <p>
                {engine
                  ? `Pergunte qualquer coisa: ${engine.name} responde usando os seus dados. Ou use um atalho.`
                  : "Escolha um atalho abaixo ou escreva o que precisa. Para conversar livremente, conecte um motor de IA."}
              </p>
              <div className="cx-welcome-grid">
                {quick.map((f) => (
                  <button key={f.id} type="button" className="cx-welcome-card" onClick={() => void startFlow(f)}>
                    <span className="cx-welcome-mod">{MODULES.find((m) => m.key === f.module)?.label}</span>
                    {f.label}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            items.map((item, index) => (
              <Bubble
                key={item.id}
                item={index === items.length - 1 ? item : { ...item, suggestions: undefined }}
                onSuggestion={(id) => {
                  const flow = flowById(id);
                  if (flow) void startFlow(flow);
                }}
                onSubmitForm={submitForm}
              />
            ))
          )}
        </div>

        {items.length > 0 && (
          <div className="cx-quick" aria-label="Atalhos">
            {quick.slice(0, 8).map((f) => (
              <button key={f.id} type="button" disabled={busy} onClick={() => void startFlow(f)}>
                {f.label}
              </button>
            ))}
          </div>
        )}

        {(voiceNotice || speaking || recorder.state === "processing") && (
          <div className="cx-voicebar" role="status">
            <span>{recorder.state === "processing" ? "Transcrevendo…" : voiceNotice}</span>
            {speaking && (
              <button type="button" className="btn-text" onClick={stopSpeaking}>
                ■ Parar voz
              </button>
            )}
          </div>
        )}

        {(attachment || uploading || attachError) && (
          <div className="cx-attach-bar">
            {uploading ? (
              <span>Enviando arquivo…</span>
            ) : attachError ? (
              <span className="cx-attach-error">{attachError}</span>
            ) : (
              attachment && (
                <span className="cx-attach-chip">
                  📎 {attachment.name}
                  <button type="button" aria-label="Remover anexo" onClick={() => setAttachment(null)}>
                    ×
                  </button>
                </span>
              )
            )}
          </div>
        )}

        <form className="cx-composer" onSubmit={submit}>
          <div className="cx-engine">
            <button type="button" className={`cx-engine-btn${engine ? " is-ai" : ""}`} onClick={() => setEngineMenu((v) => !v)} aria-expanded={engineMenu}>
              <span className="cx-engine-dot" />
              {settings === null ? "Carregando…" : engine ? engine.name : "Só atalhos"}
            </button>
            {engineMenu && settings && (
              <div className="cx-engine-menu" role="menu">
                <button type="button" role="menuitemradio" aria-checked={!engine} onClick={() => void chooseEngine("none")}>
                  <b>Só atalhos</b>
                  <span>Respostas prontas, sem IA</span>
                </button>
                {settings.providers.map((p) => (
                  <button key={p.id} type="button" role="menuitemradio" aria-checked={settings.provider === p.id} disabled={!p.available} onClick={() => void chooseEngine(p.id)}>
                    <b>{p.name}</b>
                    <span>{p.detail}</span>
                  </button>
                ))}
                <button type="button" className="cx-engine-more" onClick={() => { setEngineMenu(false); setSettingsOpen(true); }}>
                  Configurar motores…
                </button>
              </div>
            )}
          </div>
          {engine && (
            <>
              <input
                ref={fileInputRef}
                type="file"
                hidden
                onChange={(e) => void onFileChosen(e)}
                aria-hidden="true"
                tabIndex={-1}
              />
              <button
                type="button"
                className="cx-attach"
                aria-label="Anexar arquivo"
                disabled={busy || uploading}
                onClick={() => fileInputRef.current?.click()}
              >
                📎
              </button>
            </>
          )}
          {voiceOn && (
            <button
              type="button"
              className={`cx-mic${recorder.state === "recording" ? " is-recording" : ""}`}
              aria-label={recorder.state === "recording" ? "Parar gravação" : "Falar"}
              disabled={busy || recorder.state === "processing"}
              onClick={() => {
                if (recorder.state === "recording") {
                  recorder.stop();
                  return;
                }
                setVoiceNotice(null);
                stopSpeaking();
                void recorder.start();
              }}
            >
              {recorder.state === "recording" ? `● ${recorder.elapsed}s` : recorder.state === "processing" ? "…" : "🎙️"}
            </button>
          )}
          <textarea
            ref={inputRef}
            rows={1}
            value={input}
            placeholder={engine ? "Pergunte ou peça qualquer coisa…" : "Ex.: gastei 45 no mercado · contas a pagar mês que vem"}
            aria-label="Mensagem"
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                submit();
              }
            }}
          />
          {streaming ? (
            <button type="button" className="cx-send is-stop" aria-label="Parar" onClick={() => abortRef.current?.abort()}>
              ■
            </button>
          ) : (
            <button type="submit" className="cx-send" aria-label="Enviar" disabled={busy || (!input.trim() && !(engine && attachment))}>
              ↑
            </button>
          )}
        </form>
      </section>

      {settingsOpen && (
        <EngineSettings
          settings={settings}
          onClose={() => setSettingsOpen(false)}
          onChanged={() => void loadSettings()}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------- bubbles

function Bubble({
  item,
  onSuggestion,
  onSubmitForm,
}: {
  item: ChatItem;
  onSuggestion: (flowId: string) => void;
  onSubmitForm: (item: ChatItem, values: Record<string, string>, rows: Record<string, string>[]) => void;
}) {
  if (item.role === "user") {
    return (
      <div className="cx-msg is-user">
        <div className="cx-bubble">
          {item.attachment && <span className="cx-attach-tag">📎 {item.attachment.name}</span>}
          {item.voice && (
            <span className="cx-voice-tag" title="Enviado por voz">
              🎙️{" "}
            </span>
          )}
          {item.text}
        </div>
      </div>
    );
  }
  const flow = item.formFlow ? flowById(item.formFlow) : undefined;
  return (
    <div className="cx-msg is-assistant">
      {item.tools.length > 0 && <ToolTrail tools={item.tools} />}
      {item.text && (
        <div className="cx-bubble">
          <Markdown text={item.text} />
        </div>
      )}
      {item.pending && !item.text && (
        <div className="cx-bubble cx-typing" aria-label="Respondendo">
          <span />
          <span />
          <span />
        </div>
      )}
      {flow && !item.formValues && <FlowForm flow={flow} prefill={item.formPrefill} onSubmit={(v, rows) => onSubmitForm(item, v, rows)} />}
      {flow && item.formValues && <FormSummary flow={flow} values={item.formValues} rows={item.formRows ?? []} />}
      {item.card && <Card card={item.card} />}
      {item.error && (
        <div className="cx-failure" role="alert">
          <b>Não deu certo</b>
          <p>{item.error}</p>
          {item.errorDetail && item.errorDetail !== item.error && (
            <details>
              <summary>Detalhes técnicos</summary>
              <code>{item.errorDetail}</code>
            </details>
          )}
        </div>
      )}
      {item.spokenError && <p className="cx-muted">{item.spokenError}</p>}
      {item.suggestions && item.suggestions.length > 0 && !item.pending && (
        <div className="cx-suggest">
          {item.suggestions.map((id) => {
            const f = flowById(id);
            return f ? (
              <button key={id} type="button" onClick={() => onSuggestion(id)}>
                {f.label}
              </button>
            ) : null;
          })}
        </div>
      )}
    </div>
  );
}

function ToolTrail({ tools }: { tools: ToolRun[] }) {
  const [open, setOpen] = useState<string | null>(null);
  return (
    <div className="cx-tools">
      {tools.map((t) => (
        <div key={t.id} className="cx-tool-wrap">
          <button
            type="button"
            className={`cx-tool${t.done ? "" : " is-running"}${t.error ? " is-error" : ""}`}
            onClick={() => setOpen(open === t.id ? null : t.id)}
            aria-expanded={open === t.id}
          >
            <span className="cx-tool-dot" />
            {TOOL_LABELS[t.name] ?? t.name}
            {t.error ? " (erro)" : ""}
          </button>
          {open === t.id && (
            <pre className="cx-tool-detail">
              {JSON.stringify({ entrada: t.args ?? {}, ...(t.error ? { erro: t.error } : { resultado: t.result }) }, null, 2)}
            </pre>
          )}
        </div>
      ))}
    </div>
  );
}

function Card({ card }: { card: CardData }) {
  const [doneRows, setDoneRows] = useState<Record<number, string>>({});
  if (card.kind === "stats") {
    return (
      <div className="cx-card">
        {card.title && <h4>{card.title}</h4>}
        <div className="cx-stats">
          {card.items.map((s) => (
            <div key={s.label} className={s.tone ? `is-${s.tone}` : ""}>
              <span>{s.label}</span>
              <b>{s.value}</b>
            </div>
          ))}
        </div>
      </div>
    );
  }
  if (card.kind === "done") {
    return (
      <div className="cx-card cx-done">
        <span className="cx-done-check" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
            <path d="m5 12.5 4.5 4.5L19 7.5" />
          </svg>
        </span>
        <div>
          <b>{card.title}</b>
          {card.lines.map((l) => (
            <span key={l}>{l}</span>
          ))}
        </div>
      </div>
    );
  }
  if (card.kind === "image") {
    return (
      <div className="cx-card cx-image">
        <img src={card.url} alt={card.alt} loading="lazy" />
      </div>
    );
  }
  if (card.kind === "list") {
    return (
      <div className="cx-card">
        {card.title && <h4>{card.title}</h4>}
        {card.items.length === 0 ? (
          <p className="cx-muted">{card.empty ?? "Nada por aqui."}</p>
        ) : (
          <ul className="cx-list">
            {card.items.map((it, i) => (
              <li key={i} className={it.done ? "is-done" : ""}>
                <b>{it.title}</b>
                {it.subtitle && <span>{it.subtitle}</span>}
                {it.meta && <small>{it.meta}</small>}
              </li>
            ))}
          </ul>
        )}
      </div>
    );
  }
  const act = card.action;
  const runAction = async (i: number) => {
    if (!act) return;
    if (act.confirm && !window.confirm(act.confirm)) return;
    setDoneRows((d) => ({ ...d, [i]: "…" }));
    try {
      await callTool(act.tool, act.args[i]);
      setDoneRows((d) => ({ ...d, [i]: "Feito" }));
    } catch (err) {
      setDoneRows((d) => ({ ...d, [i]: err instanceof Error ? err.message : "Erro" }));
    }
  };
  return (
    <div className="cx-card">
      {card.title && <h4>{card.title}</h4>}
      {card.rows.length === 0 ? (
        <p className="cx-muted">Nada por aqui.</p>
      ) : (
        <div className="cx-table-scroll">
          <table className="cx-table">
            <thead>
              <tr>
                {card.columns.map((c) => (
                  <th key={c}>{c}</th>
                ))}
                {act && <th />}
              </tr>
            </thead>
            <tbody>
              {card.rows.map((row, i) => (
                <tr key={i} className={doneRows[i] === "Feito" ? "is-done" : ""}>
                  {row.map((cell, j) => (
                    <td key={j}>{cell}</td>
                  ))}
                  {act && (
                    <td className="cx-row-action">
                      {doneRows[i] ? (
                        <small>{doneRows[i]}</small>
                      ) : (
                        <button type="button" onClick={() => void runAction(i)}>
                          {act.label}
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {card.footer && <p className="cx-card-foot">{card.footer}</p>}
    </div>
  );
}

// ---------------------------------------------------------------- forms

function useOptions(fields: FlowField[]) {
  const [options, setOptions] = useState<Partial<Record<OptionSource, { value: string; label: string }[]>>>({});
  useEffect(() => {
    const sources = [...new Set(fields.map((f) => f.source).filter((s): s is OptionSource => !!s))];
    let alive = true;
    for (const s of sources) {
      loadOptions(s)
        .then((opts) => alive && setOptions((prev) => ({ ...prev, [s]: opts })))
        .catch(() => alive && setOptions((prev) => ({ ...prev, [s]: [] })));
    }
    return () => {
      alive = false;
    };
  }, [fields]);
  return options;
}

function initialValues(fields: FlowField[], prefill?: Record<string, string>) {
  const v: Record<string, string> = {};
  for (const f of fields) v[f.name] = prefill?.[f.name] ?? f.initial?.() ?? "";
  return v;
}

function FieldInput({
  field,
  value,
  options,
  onChange,
}: {
  field: FlowField;
  value: string;
  options?: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  const common = { value, onChange: (e: { target: { value: string } }) => onChange(e.target.value), required: field.required, "aria-label": field.label };
  switch (field.type) {
    case "select": {
      const list = field.options ?? options;
      return (
        <select {...common}>
          {!field.options && <option value="">{list ? "Escolha…" : "Carregando…"}</option>}
          {list?.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      );
    }
    case "textarea":
      return <textarea rows={3} placeholder={field.placeholder} {...common} />;
    case "money":
      return <input inputMode="decimal" placeholder={field.placeholder ?? "0,00"} {...common} />;
    case "number":
      return <input type="number" min={0} placeholder={field.placeholder} {...common} />;
    case "date":
      return <input type="date" {...common} />;
    case "datetime":
      return <input type="datetime-local" {...common} />;
    case "month":
      return <input type="month" {...common} />;
    default:
      return <input placeholder={field.placeholder} {...common} />;
  }
}

function FlowForm({ flow, prefill, onSubmit }: { flow: Flow; prefill?: Record<string, string>; onSubmit: (values: Record<string, string>, rows: Record<string, string>[]) => void }) {
  const fields = useMemo(() => flow.fields ?? [], [flow]);
  const rowFields = useMemo(() => flow.rowFields ?? [], [flow]);
  const allFields = useMemo(() => [...fields, ...rowFields], [fields, rowFields]);
  const options = useOptions(allFields);
  const [values, setValues] = useState(() => initialValues(fields, prefill));
  const [rows, setRows] = useState(() => (rowFields.length ? [initialValues(rowFields), initialValues(rowFields)] : []));
  const [error, setError] = useState<string | null>(null);

  const visible = fields.filter((f) => !f.when || f.when(values));

  const send = (e: FormEvent) => {
    e.preventDefault();
    const missing = visible.find((f) => f.required && !values[f.name]?.trim());
    const filledRows = rows.filter((r) => Object.values(r).some((v) => v.trim() && !["pix", "debito"].includes(v)));
    const rowMissing = filledRows.some((r) => rowFields.some((f) => f.required && !r[f.name]?.trim()));
    if (missing) return setError(`Preencha: ${missing.label}`);
    if (rowFields.length && filledRows.length === 0) return setError("Adicione pelo menos uma linha.");
    if (rowMissing) return setError("Complete as linhas: descrição, valor e categoria.");
    const clean = Object.fromEntries(Object.entries(values).filter(([k, v]) => v !== "" && visible.some((f) => f.name === k)));
    onSubmit(clean, filledRows);
  };

  return (
    <form className="cx-form" onSubmit={send}>
      <div className="cx-form-grid">
        {visible.map((f) => (
          <label key={f.name} className={f.type === "textarea" ? "is-wide" : ""}>
            <span>
              {f.label}
              {f.required ? "" : " (opcional)"}
            </span>
            <FieldInput field={f} value={values[f.name] ?? ""} options={f.source ? options[f.source] : undefined} onChange={(v) => setValues((prev) => ({ ...prev, [f.name]: v }))} />
          </label>
        ))}
      </div>
      {rowFields.length > 0 && (
        <div className="cx-rows">
          {rows.map((row, i) => (
            <div key={i} className="cx-row">
              {rowFields.map((f) => (
                <FieldInput
                  key={f.name}
                  field={{ ...f, required: false, placeholder: f.placeholder ?? f.label }}
                  value={row[f.name] ?? ""}
                  options={f.source ? options[f.source] : undefined}
                  onChange={(v) => setRows((prev) => prev.map((r, j) => (j === i ? { ...r, [f.name]: v } : r)))}
                />
              ))}
              <button type="button" className="icon-btn" aria-label="Remover linha" onClick={() => setRows((prev) => prev.filter((_, j) => j !== i))}>
                <IconTrash />
              </button>
            </div>
          ))}
          <button type="button" className="btn-text" onClick={() => setRows((prev) => [...prev, initialValues(rowFields)])}>
            + Linha
          </button>
        </div>
      )}
      {error && <p className="cx-error">{error}</p>}
      <button type="submit" className="btn-primary">
        Enviar
      </button>
    </form>
  );
}

function FormSummary({ flow, values, rows }: { flow: Flow; values: Record<string, string>; rows: Record<string, string>[] }) {
  const label = (f: FlowField, v: string) => f.options?.find((o) => o.value === v)?.label ?? v;
  const parts = (flow.fields ?? []).filter((f) => values[f.name] && f.source !== "openBills").map((f) => `${f.label}: ${label(f, values[f.name])}`);
  return (
    <div className="cx-form-summary">
      {parts.join(" · ")}
      {rows.length > 0 && ` · ${rows.length} ${rows.length === 1 ? "linha" : "linhas"}`}
    </div>
  );
}
