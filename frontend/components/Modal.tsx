"use client";

import { useEffect } from "react";
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

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modal-shell${wide ? " is-wide" : ""}`} onClick={(e) => e.stopPropagation()}>
        <button className="icon-btn modal-close" type="button" aria-label="Fechar" onClick={onClose}>
          <IconClose />
        </button>
        {children}
      </div>
    </div>
  );
}
