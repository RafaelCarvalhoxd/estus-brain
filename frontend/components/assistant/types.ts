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
  | { kind: "done"; title: string; lines: string[] };

export interface ChatItem {
  id: string;
  role: "user" | "assistant";
  text: string;
  tools: ToolRun[];
  card?: CardData;
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
  kind: "login" | "api" | "local";
  available: boolean;
  detail: string;
  needs_key?: boolean;
  has_key?: boolean;
  model: string;
  models?: string[];
}

export interface AssistantSettings {
  provider: string;
  ollama_url: string;
  providers: ProviderStatus[];
  mcp: { url: string; token: string };
  can_store_keys: boolean;
  multi_user: boolean;
  voice?: { available: boolean; detail: string };
}

/** The Telegram bot as the settings screen sees it (never the token). */
export interface TelegramView {
  has_token: boolean;
  from_env: boolean;
  can_store_token: boolean;
  bot_username: string;
  paired: boolean;
  owner_name: string;
  pairing_code?: string;
  pairing_expires_at?: string;
  morning_enabled: boolean;
  morning_time: string;
  evening_enabled: boolean;
  evening_time: string;
  reminders_enabled: boolean;
  events_enabled: boolean;
  events_minutes_before: number;
  status: { state: "off" | "unpaired" | "ok" | "error"; message: string };
}
