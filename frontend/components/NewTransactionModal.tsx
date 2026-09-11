"use client";

import { useState } from "react";
import type { Category, CreditCard } from "@/lib/types";
import { Modal } from "./Modal";
import { NewTransactionForm } from "./NewTransactionForm";
import { IconPlus } from "./icons";

export function NewTransactionModal({ categories, cards }: { categories: Category[]; cards: CreditCard[] }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="btn-primary" type="button" onClick={() => setOpen(true)}>
        <IconPlus />
        Novo lançamento
      </button>
      <Modal open={open} onClose={() => setOpen(false)}>
        <NewTransactionForm categories={categories} cards={cards} onSuccess={() => setOpen(false)} />
      </Modal>
    </>
  );
}
