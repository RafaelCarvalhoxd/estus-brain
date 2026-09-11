"use client";

import { useState } from "react";
import { Modal } from "./Modal";
import { VaultEntryForm } from "./VaultEntryForm";
import { IconPlus } from "./icons";

export function NewVaultEntryModal() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="btn-primary" type="button" onClick={() => setOpen(true)}>
        <IconPlus />
        Nova senha
      </button>
      <Modal open={open} onClose={() => setOpen(false)}>
        <div className="panel">
          <div className="panel-head">
            <h2>Nova senha</h2>
          </div>
          <VaultEntryForm onDone={() => setOpen(false)} />
        </div>
      </Modal>
    </>
  );
}
