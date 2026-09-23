// Shapes shared by the chat screen. They mirror the Go assistant package
// (internal/assistant) by hand, like the rest of the frontend does.

export type ModuleKey =
  | "geral"
  | "financeiro"
  | "contas"
  | "notas"
  | "lembretes"
  | "agenda"
  | "habitos"
  | "treino"
  | "dieta"
  | "documentos";

export interface ToolRun {
  id: string;
  name: string;
  args?: unknown;
  result?: unknown;
  error?: string;
  done: boolean;
}

/** A ready-made flow's answer, kept with the message so history re-renders it. */
export type CardData =
  | { kind: "stats"; title?: string; items: { label: string; value: string; tone?: "good" | "bad" | "muted" }[] }
  | {
      kind: "table";
      title?: string;
      columns: string[];
      rows: string[][];
      footer?: string;
      action?: { label: string; tool: string; confirm?: string; args: Record<string, unknown>[] };
    }
  | { kind: "list"; title?: string; items: { title: string; subtitle?: string; meta?: string; done?: boolean }[]; empty?: string }
  | { kind: "done"; title: string; lines: string[] }
  | { kind: "image"; url: string; alt: string };

/** A file attached to a message — set client-side when sending, or read
 * back from a stored message's `data.attachment` on history reload. */
export interface ChatAttachment {
  name: string;
  content_type: string;
}

export interface ChatItem {
  id: string;
  role: "user" | "assistant";
  text: string;
  tools: ToolRun[];
  card?: CardData;
  attachment?: ChatAttachment;
  /** A form waiting to be filled, for the flow with this id. */
  formFlow?: string;
  formPrefill?: Record<string, string>;
  /** Set once the form was sent: the values, shown as a summary. */
  formValues?: Record<string, string>;
  formRows?: Record<string, string>[];
  /** Flow ids offered as next steps. */
  suggestions?: string[];
  provider?: string;
  /** Why it failed, in words the person can act on. */
  error?: string;
  /** The raw error behind `error`, shown folded as "Detalhes técnicos". */
  errorDetail?: string;
  /** Sent by voice: the answer to it is read aloud. */
  voice?: boolean;
  /** Set when the answer couldn't be read aloud. */
  spokenError?: string;
  pending?: boolean;
}

export interface ConversationSummary {
  id: string;
  title: string;
  module: string;
  provider: string;
  updated_at: string;
}

export interface ProviderStatus {
  id: string;
  name: string;
  available: boolean;
  detail: string;
  model: string;
  models?: string[];
}

export interface AssistantSettings {
  /** "agent" when the chat answers through the external agent, "none" for shortcuts only. */
  provider: string;
  providers: ProviderStatus[];
  /** Saved values only; env_* is what .env gives for whatever is left empty. */
  agent: { url: string; model: string; has_token: boolean; env_url: string; env_model: string; env_token: boolean };
  mcp: { url: string; token: string };
  can_store_keys: boolean;
}

