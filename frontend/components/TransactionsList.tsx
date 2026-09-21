"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { Category, Transaction, PaymentMethod } from "@/lib/types";
import { formatYearMonthShort } from "@/lib/month";
import { updateTransactionAction, deleteTransactionAction } from "@/app/financeiro/lancamentos/actions";
import { IconPencil, IconTrash } from "./icons";

const PAYMENT_LABEL: Record<PaymentMethod, string> = {
  debito: "débito",
  credito: "crédito",
  pix: "pix",
};

function meta(t: Transaction): string {
  const parts = [t.category_name, PAYMENT_LABEL[t.payment_method]];
  if (t.payment_method === "credito") {
    parts.push(`comprado em ${formatDay(t.purchase_date)}`);
  } else {
    parts.push(formatDay(t.purchase_date));
  }
  if (t.is_recurring) parts.push("recorrente");
  return parts.join(" · ");
}

function formatDay(iso: string): string {
  const [, month, day] = iso.split("-");
  return `${day}/${month}`;
}

function EditRow({
  transaction,
  categories,
  onDone,
}: {
  transaction: Transaction;
  categories: Category[];
  onDone: () => void;
}) {
  const router = useRouter();
  const [description, setDescription] = useState(transaction.description);
  const [categoryId, setCategoryId] = useState(transaction.category_id);
  const [saving, setSaving] = useState(false);

  async function save() {
    setSaving(true);
    try {
      await updateTransactionAction(transaction.id, description, categoryId);
      router.refresh();
      onDone();
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="txn txn-editing">
      <div className="row2" style={{ flex: 1 }}>
        <input
          className="txn-edit-input"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          disabled={saving}
        />
        <select
          className="txn-edit-input"
          value={categoryId}
          onChange={(e) => setCategoryId(e.target.value)}
          disabled={saving}
        >
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </div>
      <div className="row-actions">
        <button className="btn-text" type="button" onClick={onDone} disabled={saving}>
          Cancelar
        </button>
        <button className="btn-text" type="button" onClick={save} disabled={saving || !description.trim()}>
          {saving ? "Salvando…" : "Salvar"}
        </button>
      </div>
    </div>
  );
}

function subtotal(items: Transaction[]): string {
  const cents = items.reduce((sum, t) => sum + t.amount.cents, 0);
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

export function TransactionsList({
  transactions,
  categories,
  month,
}: {
  transactions: Transaction[];
  categories: Category[];
  month: string;
}) {
  const router = useRouter();
  const [categoryFilter, setCategoryFilter] = useState("");
  const [methodFilter, setMethodFilter] = useState<"" | PaymentMethod>("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<{ id: string; message: string } | null>(null);

  const filtered = useMemo(
    () =>
      transactions.filter(
        (t) =>
          (categoryFilter === "" || t.category_id === categoryFilter) &&
          (methodFilter === "" || t.payment_method === methodFilter),
      ),
    [transactions, categoryFilter, methodFilter],
  );

  const usedCategories = useMemo(() => {
    const ids = new Set(transactions.map((t) => t.category_id));
    return categories.filter((c) => ids.has(c.id));
  }, [transactions, categories]);

  const fixed = useMemo(() => filtered.filter((t) => t.is_recurring), [filtered]);
  const variable = useMemo(() => filtered.filter((t) => !t.is_recurring), [filtered]);

  async function handleDelete(id: string) {
    if (!confirm("Excluir este lançamento? Não dá para desfazer.")) return;
    setDeletingId(id);
    setDeleteError(null);
    try {
      const result = await deleteTransactionAction(id);
      if (result.error) {
        setDeleteError({ id, message: result.error });
        return;
      }
      router.refresh();
    } finally {
      setDeletingId(null);
    }
  }

  function renderRow(t: Transaction) {
    if (editingId === t.id) {
      return <EditRow key={t.id} transaction={t} categories={categories} onDone={() => setEditingId(null)} />;
    }
    return (
      <div className="txn" key={t.id}>
        <span className="txn-dot" style={{ background: t.category_color }} />
        <div className="txn-main">
          <div className="txn-title">
            {t.description}
            {t.installment_total && t.installment_total > 1
              ? ` — parcela ${t.installment_number}/${t.installment_total}`
              : ""}
          </div>
          <div className="txn-meta">{meta(t)}</div>
          {deleteError?.id === t.id && <p className="form-error">{deleteError.message}</p>}
        </div>
        <div className="txn-amt">
          <div className="txn-value tab">{t.amount.formatted}</div>
          {t.payment_method === "credito" && <span className="badge">fatura {formatYearMonthShort(month)}</span>}
        </div>
        <div className="row-actions">
          <button className="icon-btn" type="button" aria-label="Editar" onClick={() => setEditingId(t.id)}>
            <IconPencil />
          </button>
          <button
            className="icon-btn bad"
            type="button"
            aria-label="Excluir"
            disabled={deletingId === t.id}
            onClick={() => handleDelete(t.id)}
          >
            <IconTrash />
          </button>
        </div>
      </div>
    );
  }

  function renderSection(title: string, items: Transaction[]) {
    if (items.length === 0) return null;
    return (
      <div className="txn-section">
        <div className="txn-section-head">
          <h3>{title}</h3>
          <span className="tab">{subtotal(items)}</span>
        </div>
        {items.map(renderRow)}
      </div>
    );
  }

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Lançamentos de {formatYearMonthShort(month)}</h2>
        <span>
          {filtered.length} de {transactions.length}
        </span>
      </div>

      {transactions.length > 0 && (
        <div className="list-filters">
          <select value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)}>
            <option value="">Todas as categorias</option>
            {usedCategories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
          <select value={methodFilter} onChange={(e) => setMethodFilter(e.target.value as "" | PaymentMethod)}>
            <option value="">Todas as formas</option>
            <option value="debito">Débito</option>
            <option value="credito">Crédito</option>
            <option value="pix">Pix</option>
          </select>
        </div>
      )}

      {transactions.length === 0 ? (
        <p className="empty-note">Nenhum lançamento neste mês ainda. Use o formulário ao lado para começar.</p>
      ) : filtered.length === 0 ? (
        <p className="empty-note">Nenhum lançamento bate com esse filtro.</p>
      ) : (
        <>
          {renderSection("Fixos", fixed)}
          {renderSection("Variáveis", variable)}
        </>
      )}
    </div>
  );
}
