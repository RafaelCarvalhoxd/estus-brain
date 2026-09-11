"use client";

import { useState } from "react";
import { Modal } from "./Modal";
import { NewEventForm } from "./EventForm";
import { IconPlus } from "./icons";

export function NewEventModal() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="btn-primary" type="button" onClick={() => setOpen(true)}>
        <IconPlus />
        Novo evento
      </button>
      <Modal open={open} onClose={() => setOpen(false)}>
        <NewEventForm onSuccess={() => setOpen(false)} />
      </Modal>
    </>
  );
}
