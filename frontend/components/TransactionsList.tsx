"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { Category, CreditCard, Transaction, PaymentMethod } from "@/lib/types";
import { formatYearMonthShort } from "@/lib/month";
import { deleteTransactionAction } from "@/app/financeiro/lancamentos/actions";
import { Modal } from "./Modal";
import { NewTransactionForm } from "./NewTransactionForm";
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
  if (t.category_kind === "receita") parts.push("receita");
  return parts.join(" · ");
}

function formatDay(iso: string): string {
  const [, month, day] = iso.split("-");
  return `${day}/${month}`;
}

function subtotal(items: Transaction[]): string {
  const cents = items.reduce((sum, t) => sum + t.amount.cents, 0);
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

export function TransactionsList({
  transactions,
  categories,
  cards,
  month,
}: {
  transactions: Transaction[];
  categories: Category[];
  cards: CreditCard[];
  month: string;
}) {
  const router = useRouter();
  const [categoryFilter, setCategoryFilter] = useState("");
  const [methodFilter, setMethodFilter] = useState<"" | PaymentMethod>("");
  const [editing, setEditing] = useState<Transaction | null>(null);
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

  // Installments are fixed until the last one: each exists only in its month.
  const isFixed = (t: Transaction) => t.is_recurring || (t.installment_total ?? 1) > 1;
  const fixed = useMemo(() => filtered.filter(isFixed), [filtered]);
  const variable = useMemo(() => filtered.filter((t) => !isFixed(t)), [filtered]);

  async function handleDelete(t: Transaction) {
    const id = t.id;
    const total = t.installment_total ?? 1;
    const message =
      total > 1
        ? `Excluir esta compra e as ${total} parcelas dela (${t.purchase_total.formatted})? Não dá para desfazer.`
        : "Excluir este lançamento? Não dá para desfazer.";
    if (!confirm(message)) return;
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
          {t.invoice_month && <span className="badge">fatura {formatYearMonthShort(t.invoice_month)}</span>}
        </div>
        <div className="row-actions">
          <button className="icon-btn" type="button" aria-label="Editar" onClick={() => setEditing(t)}>
            <IconPencil />
          </button>
          <button
            className="icon-btn bad"
            type="button"
            aria-label="Excluir"
            disabled={deletingId === t.id}
            onClick={() => handleDelete(t)}
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
      {editing && (
        <Modal open onClose={() => setEditing(null)}>
          <NewTransactionForm
            key={editing.id}
            categories={categories}
            cards={cards}
            editing={editing}
            onSuccess={() => {
              setEditing(null);
              router.refresh();
            }}
          />
        </Modal>
      )}
    </div>
  );
}
