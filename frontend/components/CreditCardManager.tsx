"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { CreditCard } from "@/lib/types";
import type { CreditCardInput } from "@/lib/api";
import {
  createCreditCardAction,
  updateCreditCardAction,
  deleteCreditCardAction,
} from "@/app/financeiro/cartoes/actions";
import { Modal } from "./Modal";
import { IconPencil, IconTrash } from "./icons";

// closingDay/dueDay are kept as text, not numbers, so the field can sit
// empty while the owner is typing instead of snapping to 0 or NaN.
type FormState = { name: string; closingDay: string; dueDay: string };

function toInput(form: FormState): CreditCardInput {
  return {
    name: form.name,
    closing_day: Number(form.closingDay),
    due_day: Number(form.dueDay),
  };
}

function CreditCardForm({
  initial,
  onSaved,
  onCancel,
  save,
}: {
  initial: FormState;
  onSaved: () => void;
  onCancel?: () => void;
  save: (input: CreditCardInput) => Promise<{ error?: string }>;
}) {
  const router = useRouter();
  const [form, setForm] = useState<FormState>(initial);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSave() {
    setSaving(true);
    setError(null);
    const result = await save(toInput(form));
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onSaved();
  }

  return (
    <div className="form-grid">
      <div className="field">
        <label htmlFor="card-name">Nome</label>
        <input
          id="card-name"
          type="text"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          disabled={saving}
        />
      </div>
      <div className="row2">
        <div className="field">
          <label htmlFor="card-closing-day">Fecha no dia</label>
          <input
            id="card-closing-day"
            type="number"
            min={1}
            max={28}
            value={form.closingDay}
            onChange={(e) => setForm({ ...form, closingDay: e.target.value })}
            disabled={saving}
          />
        </div>
        <div className="field">
          <label htmlFor="card-due-day">Vence no dia</label>
          <input
            id="card-due-day"
            type="number"
            min={1}
            max={28}
            value={form.dueDay}
            onChange={(e) => setForm({ ...form, dueDay: e.target.value })}
            disabled={saving}
          />
        </div>
      </div>
      <p className="empty-note">
        Mudar esses dias não altera lançamentos já feitos — só vale para os próximos.
      </p>
      {error && <p className="form-error">{error}</p>}
      <div className="row-actions" style={{ justifyContent: "flex-end" }}>
        {onCancel && (
          <button className="btn-text" type="button" onClick={onCancel} disabled={saving}>
            Cancelar
          </button>
        )}
        <button className="btn-block" type="button" onClick={handleSave} disabled={saving || !form.name.trim()}>
          {saving ? "Salvando…" : "Salvar"}
        </button>
      </div>
    </div>
  );
}

function CreditCardRow({ card }: { card: CreditCard }) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  async function handleDelete() {
    if (!confirm(`Excluir o cartão "${card.name}"? Só é possível se não houver lançamentos nele.`)) return;
    setDeleting(true);
    setDeleteError(null);
    const result = await deleteCreditCardAction(card.id);
    setDeleting(false);
    if (result.error) {
      setDeleteError(result.error);
      return;
    }
    router.refresh();
  }

  if (editing) {
    return (
      <div className="credit-card-row category-manager-row-editing">
        <CreditCardForm
          initial={{
            name: card.name,
            closingDay: String(card.closing_day),
            dueDay: String(card.due_day),
          }}
          onSaved={() => setEditing(false)}
          onCancel={() => setEditing(false)}
          save={(input) => updateCreditCardAction(card.id, input)}
        />
      </div>
    );
  }

  return (
    <div className="credit-card-row">
      <span className="credit-card-name">{card.name}</span>
      <span className="credit-card-cycle">
        Fecha dia {card.closing_day} · vence dia {card.due_day}
      </span>
      <div className="row-actions">
        <button className="icon-btn" type="button" aria-label="Editar cartão" onClick={() => setEditing(true)}>
          <IconPencil />
        </button>
        <button
          className="icon-btn bad"
          type="button"
          aria-label="Excluir cartão"
          disabled={deleting}
          onClick={handleDelete}
        >
          <IconTrash />
        </button>
      </div>
      {deleteError && <p className="form-error credit-card-error">{deleteError}</p>}
    </div>
  );
}

export function CreditCardManager({ cards }: { cards: CreditCard[] }) {
  const [creating, setCreating] = useState(false);

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Cartões</h2>
        <button className="btn-text" type="button" onClick={() => setCreating(true)}>
          + Novo cartão
        </button>
      </div>

      {cards.length === 0 ? (
        <p className="empty-note">Nenhum cartão ainda.</p>
      ) : (
        cards.map((c) => <CreditCardRow key={c.id} card={c} />)
      )}

      <Modal open={creating} onClose={() => setCreating(false)}>
        <div className="panel">
          <div className="panel-head">
            <h2>Novo cartão</h2>
          </div>
          <CreditCardForm
            initial={{ name: "", closingDay: "", dueDay: "" }}
            onSaved={() => setCreating(false)}
            save={(input) => createCreditCardAction(input)}
          />
        </div>
      </Modal>
    </div>
  );
}
