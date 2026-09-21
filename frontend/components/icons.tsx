// Minimal stroke icon set for the sidebar nav. Kept as one file since
// they're small, purely decorative-but-meaningful (each names a real
// section), and never reused outside the rail.

// The home "núcleo": a side-view brain with a circuit trace through it —
// the same picture the home page animates, reduced to a glyph.
export function IconBrain() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4.2 13.6C2.9 11 4 7.6 6.6 6.4 7.7 4.2 10.4 3.2 12.8 3.8c2.6-.6 5.3.8 6.2 3.2 2 .9 2.8 3.4 1.9 5.4.4 2-.9 3.9-2.9 4.2-1.2 1.2-3.1 1.4-4.5.6-1.6.8-3.6.5-4.8-.8-2.1.1-4-1.1-4.5-2.8Z" />
      <path d="M13.2 17.2 13 20.5" />
      <path d="M8.2 10.2h3l1.6-2.4h2.4" />
      <path d="M11.2 10.2l1.4 2.6h3.2" />
      <circle cx="15.8" cy="7.8" r="0.6" fill="currentColor" />
      <circle cx="16.4" cy="12.8" r="0.6" fill="currentColor" />
    </svg>
  );
}

export function IconList() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <line x1="4" y1="7" x2="20" y2="7" />
      <line x1="4" y1="12" x2="20" y2="12" />
      <line x1="4" y1="17" x2="14" y2="17" />
    </svg>
  );
}

export function IconCard() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2.5" y="5" width="19" height="14" rx="2.5" />
      <line x1="2.5" y1="9.5" x2="21.5" y2="9.5" />
    </svg>
  );
}

export function IconTag() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 12.5 12.7 19.8a2 2 0 0 1-2.8 0l-6.7-6.7a2 2 0 0 1-.6-1.4V5a2 2 0 0 1 2-2h6.7c.5 0 1 .2 1.4.6l6.7 6.7a2 2 0 0 1 0 2.8Z" />
      <circle cx="7.5" cy="7.5" r="1.2" />
    </svg>
  );
}

export function IconRepeat() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="17 2 21 6 17 10" />
      <path d="M3 12V9a3 3 0 0 1 3-3h15" />
      <polyline points="7 22 3 18 7 14" />
      <path d="M21 12v3a3 3 0 0 1-3 3H3" />
    </svg>
  );
}

export function IconLock() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2 4 5v6c0 5 3.4 8.6 8 11 4.6-2.4 8-6 8-11V5Z" />
    </svg>
  );
}

export function IconChevronLeft() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="15 18 9 12 15 6" />
    </svg>
  );
}

export function IconChevronRight() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

export function IconWallet() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="6" width="18" height="14" rx="2.5" />
      <path d="M3 10h13a3 3 0 0 1 0 6H3" />
      <circle cx="16" cy="13" r="1.2" />
    </svg>
  );
}

export function IconNote() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 3h10l4 4v14H5Z" />
      <path d="M15 3v4h4" />
      <line x1="8" y1="12" x2="16" y2="12" />
      <line x1="8" y1="16" x2="13" y2="16" />
    </svg>
  );
}

export function IconBell() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 9a6 6 0 0 1 12 0c0 4 1.5 5.5 2 6H4c.5-.5 2-2 2-6Z" />
      <path d="M10 19a2 2 0 0 0 4 0" />
    </svg>
  );
}

export function IconCalendar() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="4.5" width="18" height="16" rx="2" />
      <line x1="3" y1="9.5" x2="21" y2="9.5" />
      <line x1="8" y1="2.5" x2="8" y2="6.5" />
      <line x1="16" y1="2.5" x2="16" y2="6.5" />
    </svg>
  );
}

export function IconPlus() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.3" strokeLinecap="round" strokeLinejoin="round">
      <line x1="12" y1="5" x2="12" y2="19" />
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

export function IconSun() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="4.2" />
      <path d="M12 2.5v2.4M12 19.1v2.4M4.6 4.6l1.7 1.7M17.7 17.7l1.7 1.7M2.5 12h2.4M19.1 12h2.4M4.6 19.4l1.7-1.7M17.7 6.3l1.7-1.7" />
    </svg>
  );
}

export function IconMoon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a6.8 6.8 0 0 0 10.5 10.5Z" />
    </svg>
  );
}

export function IconMonitor() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2.5" y="4" width="19" height="13" rx="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
    </svg>
  );
}

export function IconPencil() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 20.5 4.6 17 16.4 5.2a1.8 1.8 0 0 1 2.6 0l1.2 1.2a1.8 1.8 0 0 1 0 2.6L8.4 20.8 4 20.5Z" />
      <line x1="14.5" y1="7.1" x2="18.3" y2="10.9" />
    </svg>
  );
}

export function IconTrash() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4.5 6.5h15" />
      <path d="M8 6.5V4.8A1.3 1.3 0 0 1 9.3 3.5h5.4a1.3 1.3 0 0 1 1.3 1.3V6.5" />
      <path d="M6.5 6.5 7.3 19.5A1.5 1.5 0 0 0 8.8 21h6.4a1.5 1.5 0 0 0 1.5-1.5l.8-13" />
      <line x1="10.2" y1="10.5" x2="10.2" y2="17" />
      <line x1="13.8" y1="10.5" x2="13.8" y2="17" />
    </svg>
  );
}

export function IconClose() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <line x1="6" y1="6" x2="18" y2="18" />
      <line x1="18" y1="6" x2="6" y2="18" />
    </svg>
  );
}

export function IconDumbbell() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6.5 6.5v11M17.5 6.5v11M3.5 9.5v5M20.5 9.5v5M6.5 12h11" />
    </svg>
  );
}

export function IconUtensils() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 3v5a2 2 0 0 0 4 0V3M7 10v11" />
      <path d="M18 3c-2 1.3-3 3.4-3 6.2V13h3v8" />
    </svg>
  );
}

export function IconUser() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="8.5" r="4" />
      <path d="M4.5 20.5c1.2-3.8 4-5.8 7.5-5.8s6.3 2 7.5 5.8" />
    </svg>
  );
}

export function IconFolder() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4.2c.5 0 1 .24 1.3.64L11.2 8h8.3A1.5 1.5 0 0 1 21 9.5v8A1.5 1.5 0 0 1 19.5 19h-15A1.5 1.5 0 0 1 3 17.5Z" />
    </svg>
  );
}

export function IconFile() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 3h8l4 4v14H6Z" />
      <path d="M14 3v4h4" />
    </svg>
  );
}

export function IconUpload() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 16V4" />
      <path d="m7.5 8.5 4.5-4.5 4.5 4.5" />
      <path d="M4 16v2.5A1.5 1.5 0 0 0 5.5 20h13a1.5 1.5 0 0 0 1.5-1.5V16" />
    </svg>
  );
}

export function IconDownload() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 4v12" />
      <path d="m7.5 11.5 4.5 4.5 4.5-4.5" />
      <path d="M4 16v2.5A1.5 1.5 0 0 0 5.5 20h13a1.5 1.5 0 0 0 1.5-1.5V16" />
    </svg>
  );
}

export function IconGear() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

export function IconSearch() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10.5" cy="10.5" r="6.5" />
      <line x1="15.3" y1="15.3" x2="20.5" y2="20.5" />
    </svg>
  );
}

// Quadros: a tiny flow — two blocks joined by a connector.
export function IconFlow() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="4" width="7" height="6" rx="1.6" />
      <rect x="14" y="14" width="7" height="6" rx="1.6" />
      <path d="M6.5 10v3.5a2 2 0 0 0 2 2H14" />
    </svg>
  );
}

// Hábitos: a check inside a loop — something done, again and again.
export function IconHabit() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 12a8 8 0 1 1-2.34-5.66" />
      <path d="M20 4v4h-4" />
      <path d="m8.5 12 2.5 2.5 4.5-5" />
    </svg>
  );
}

// Anexo: um clipe de papel — o mesmo símbolo universal de anexar arquivo.
export function IconPaperclip() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8.5 12.5V7a3.5 3.5 0 0 1 7 0v9a5.5 5.5 0 0 1-11 0V8.5" />
    </svg>
  );
}

// Voz: um microfone — falar com o chat em vez de digitar.
export function IconMic() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="2.5" width="6" height="11" rx="3" />
      <path d="M5.5 11a6.5 6.5 0 0 0 13 0" />
      <line x1="12" y1="17.5" x2="12" y2="21" />
      <line x1="8.5" y1="21" x2="15.5" y2="21" />
    </svg>
  );
}

// Voz gravando: um quadrado — o botão que interrompe a gravação em andamento.
export function IconStop() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" stroke="none">
      <rect x="6" y="6" width="12" height="12" rx="2" />
    </svg>
  );
}

// Conversa: a speech bubble with a spark — talking to the brain.
export function IconChat() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 12.5a7.5 7.5 0 0 1-10.9 6.7L4 20.5l1.4-4.6A7.5 7.5 0 1 1 20 12.5Z" />
      <path d="M12.5 9v3.5M10.75 10.75h3.5" />
    </svg>
  );
}
