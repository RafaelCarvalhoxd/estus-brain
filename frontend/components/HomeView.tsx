"use client";

import { createContext, useEffect, useState, type ReactNode } from "react";

// Whether the overview is open on the home screen. The brain listens so it
// can stop drawing once it has fallen out of view.
export const HomeViewContext = createContext(false);

function EyeClosed() {
  return (
    <svg className="eye-closed" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 10.5c2.3 3 5.3 4.5 9 4.5s6.7-1.5 9-4.5" />
      <path d="M6.3 13.7 4.8 16M12 15v2.7M17.7 13.7l1.5 2.3" />
    </svg>
  );
}

function EyeOpen() {
  return (
    <svg className="eye-open" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

function ChatGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 12.5a7.5 7.5 0 0 1-10.9 6.7L4 20.5l1.4-4.6A7.5 7.5 0 1 1 20 12.5Z" />
      <path d="M12.5 9v3.5M10.75 10.75h3.5" />
    </svg>
  );
}

type View = "brain" | "dash" | "chat";

// The home screen's three states: the brain; behind the eye, the overview of
// every module, which drops the brain out of the bottom of the screen and
// falls into place; and behind the button under the brain, the chat, which
// does the opposite — the brain lifts away and the chat rises from below.
export function HomeView({
  hub,
  dashboard,
  chat,
  controls,
  initialView = "brain",
}: {
  hub: ReactNode;
  dashboard: ReactNode;
  chat: ReactNode;
  controls: ReactNode;
  initialView?: View;
}) {
  const [view, setView] = useState<View>(initialView);

  useEffect(() => {
    if (view === "brain") return;
    const onKey = (e: KeyboardEvent) => {
      // A dialog inside the chat closes itself first.
      if (e.key === "Escape" && !document.querySelector(".modal-overlay")) setView("brain");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [view]);

  const dashOpen = view === "dash";
  const chatOpen = view === "chat";

  return (
    <HomeViewContext.Provider value={view !== "brain"}>
      <div className="home" data-view={view}>
        <header className="home-top">
          <span aria-hidden="true" />
          <button
            type="button"
            className="eye-btn"
            aria-pressed={dashOpen}
            aria-label={dashOpen ? "Fechar o painel geral" : "Abrir o painel geral"}
            title={dashOpen ? "Fechar o painel geral" : "Abrir o painel geral"}
            onClick={() => setView(dashOpen ? "brain" : "dash")}
          >
            <EyeClosed />
            <EyeOpen />
          </button>
          <div className="home-top-end">{controls}</div>
        </header>

        <main className="home-main">
          <h1 className="sr-only">Núcleo</h1>
          {hub}
          <section className="dash" aria-label="Painel geral" aria-hidden={!dashOpen} inert={!dashOpen}>
            {dashboard}
          </section>
        </main>

        <button
          type="button"
          className="chat-launch"
          aria-label="Conversar com o cérebro"
          tabIndex={view === "brain" ? 0 : -1}
          onClick={() => setView("chat")}
        >
          <ChatGlyph />
          <span>Conversar</span>
        </button>

        <section className="chatlay" aria-label="Conversa" aria-hidden={!chatOpen} inert={!chatOpen}>
          <button type="button" className="chatlay-close" onClick={() => setView("brain")} aria-label="Voltar ao cérebro">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="m6 9 6 6 6-6" />
            </svg>
            Voltar ao cérebro
          </button>
          {chat}
        </section>
      </div>
    </HomeViewContext.Provider>
  );
}
