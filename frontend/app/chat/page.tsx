import Link from "next/link";
import { getConversation, listConversations } from "@/lib/assistant";
import { ChatApp } from "@/components/assistant/ChatApp";
import { BrainBackdrop } from "@/components/brain/BrainBackdrop";
import { ThemeToggle } from "@/components/ThemeToggle";
import { IconBrain, IconChevronRight } from "@/components/icons";
import "../ui.css";
import "./chat.css";

export default async function ChatPage({ searchParams }: { searchParams: Promise<{ c?: string }> }) {
  const { c } = await searchParams;
  const [conversations, conversation] = await Promise.all([listConversations(), c ? getConversation(c) : null]);

  return (
    <div className="shell chat-shell">
      <BrainBackdrop module={null} />
      <header className="modbar">
        <div className="modbar-crumbs">
          <Link href="/" className="modbar-home" aria-label="Voltar ao núcleo">
            <IconBrain />
          </Link>
          <Link href="/" className="modbar-back">
            Núcleo
          </Link>
          <IconChevronRight />
          <span className="modbar-current chat-crumb" aria-current="page">
            Conversa
          </span>
        </div>
        <ThemeToggle />
      </header>
      <main className="main chat-main">
        <ChatApp initialConversations={conversations} initialConversation={conversation} />
      </main>
    </div>
  );
}
