"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";
import { IconClose } from "./icons";

export function Modal({
  open,
  onClose,
  children,
  wide = false,
}: {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  /** For editors with rows of fields side by side. */
  wide?: boolean;
}) {
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  // Modals only open after a click, so there is no server render to match.
  if (!open || typeof document === "undefined") return null;

  // Portaled to <body>: a panel's backdrop-filter makes it the containing
  // block for position: fixed, which would trap the overlay inside it.
  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modal-shell${wide ? " is-wide" : ""}`} onClick={(e) => e.stopPropagation()}>
        <button className="icon-btn modal-close" type="button" aria-label="Fechar" onClick={onClose}>
          <IconClose />
        </button>
        {children}
      </div>
    </div>,
    document.body,
  );
}
