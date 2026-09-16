"use client";

import { useState, useTransition } from "react";
import { renameBoardAction } from "@/app/quadros/actions";

// The board's name in the breadcrumb, renamed in place: click, type, Enter.
export function BoardTitle({ boardId, name }: { boardId: string; name: string }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(name);
  const [pending, startTransition] = useTransition();

  const commit = () => {
    setEditing(false);
    const next = value.trim();
    if (!next || next === name) {
      setValue(name);
      return;
    }
    startTransition(async () => {
      const result = await renameBoardAction(boardId, next);
      if (result.error) setValue(name);
    });
  };

  if (editing) {
    return (
      <input
        className="board-title-input"
        value={value}
        maxLength={120}
        aria-label="Nome do quadro"
        autoFocus
        onChange={(e) => setValue(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
          if (e.key === "Escape") {
            setValue(name);
            setEditing(false);
          }
        }}
      />
    );
  }

  return (
    <button
      type="button"
      className="board-title"
      aria-current="page"
      title="Renomear quadro"
      disabled={pending}
      onClick={() => setEditing(true)}
    >
      {value}
    </button>
  );
}
