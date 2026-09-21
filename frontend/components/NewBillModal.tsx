"use client";

import { useState } from "react";
import type { Category, CreditCard } from "@/lib/types";
import { Modal } from "./Modal";
import { BillForm } from "./BillForm";
import { IconPlus } from "./icons";

export function NewBillModal({ categories, cards }: { categories: Category[]; cards: CreditCard[] }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="btn-primary" type="button" onClick={() => setOpen(true)}>
        <IconPlus />
        Nova conta
      </button>
      <Modal open={open} onClose={() => setOpen(false)}>
        <BillForm categories={categories} cards={cards} onSuccess={() => setOpen(false)} />
      </Modal>
    </>
  );
}
