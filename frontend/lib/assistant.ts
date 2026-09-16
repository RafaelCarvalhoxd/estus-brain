import "server-only";
import type { ConversationSummary } from "@/components/assistant/types";

// Server-side reads for the chat's first paint; everything after goes through
// the /api/assistant proxy from the browser.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

export interface StoredMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  data: { card?: unknown; tool_calls?: { id: string; name: string; args?: unknown; result?: unknown; error?: string }[]; error?: string };
  provider: string;
  created_at: string;
}

export async function listConversations(): Promise<ConversationSummary[]> {
  const res = await fetch(`${API_URL}/api/assistant/conversations`, { cache: "no-store" });
  if (!res.ok) throw new Error(`estus-vault api /api/assistant/conversations -> ${res.status}`);
  return res.json() as Promise<ConversationSummary[]>;
}

export async function getConversation(id: string): Promise<{ conversation: ConversationSummary; messages: StoredMessage[] } | null> {
  const res = await fetch(`${API_URL}/api/assistant/conversations/${encodeURIComponent(id)}`, { cache: "no-store" });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`estus-vault api /api/assistant/conversations/${id} -> ${res.status}`);
  return res.json() as Promise<{ conversation: ConversationSummary; messages: StoredMessage[] }>;
}
