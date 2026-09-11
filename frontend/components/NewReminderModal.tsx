"use client";

import { useState } from "react";
import { Modal } from "./Modal";
import { NewReminderForm } from "./ReminderList";
import { IconPlus } from "./icons";

export function NewReminderModal() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="btn-primary" type="button" onClick={() => setOpen(true)}>
        <IconPlus />
        Novo lembrete
      </button>
      <Modal open={open} onClose={() => setOpen(false)}>
        <NewReminderForm onSuccess={() => setOpen(false)} />
      </Modal>
    </>
  );
}
