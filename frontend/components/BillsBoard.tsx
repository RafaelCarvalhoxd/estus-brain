"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { Bill, BillDirection, BillPaymentMethod, BillStatus } from "@/lib/bills";
import type { Category } from "@/lib/types";
import {
  markBillPaidAction,
  updateBillAction,
  deleteBillAction,
  endSeriesAction,
  resumeSeriesAction,
} from "@/app/financeiro/contas/actions";
import { IconPencil, IconTrash } from "./icons";

function statusPillClass(status: BillStatus): string {
  if (status === "atrasado") return "pill bad";
  if (status === "pago" || status === "recebido") return "pill good";
  return "pill";
}

function statusLabel(status: BillStatus): string {
  switch (status) {
    case "atrasado":
      return "Atrasado";
    case "pago":
      return "Pago";
    case "recebido":
      return "Recebido";
    default:
      return "Pendente";
  }
}

function formatDueDate(dueDate: string): string {
  const [year, month, day] = dueDate.split("-");
  return `${day}/${month}/${year}`;
}

function EditBillRow({ bill, categories, onDone }: { bill: Bill; categories: Category[]; onDone: () => void }) {
  const router = useRouter();
  const [description, setDescription] = useState(bill.description);
  const [amount, setAmount] = useState((bill.amount.cents / 100).toFixed(2).replace(".", ","));
  const [dueDate, setDueDate] = useState(bill.due_date);
  const [categoryId, setCategoryId] = useState(bill.category_id ?? "");
  const [paymentMethod, setPaymentMethod] = useState<BillPaymentMethod | "">(bill.payment_method ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Uma conta só entra ou sai de uma série na criação (BillService.Update
  // nunca muda SeriesID) — bill.recurring aqui é só leitura, não um toggle.
  const needsSeriesFields = bill.recurring && bill.direction === "pagar";

  async function save() {
    setSaving(true);
    setError(null);
    const result = await updateBillAction(bill.id, {
      description,
      amount,
      due_date: dueDate,
      direction: bill.direction,
      category_id: categoryId || undefined,
      recurring: bill.recurring,
      payment_method: paymentMethod || undefined,
      amount_varies: bill.amount_varies,
    });
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onDone();
  }

  return (
    <div className="bill-row bill-row-editing">
      <div className="bill-edit-grid">
        <input
          className="txn-edit-input"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          disabled={saving}
        />
        <input
          className="txn-edit-input"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          inputMode="decimal"
          disabled={saving}
        />
        <input
          className="txn-edit-input"
          type="date"
          value={dueDate}
          onChange={(e) => setDueDate(e.target.value)}
          disabled={saving}
        />
        <select
          className="txn-edit-input"
          value={categoryId}
          onChange={(e) => setCategoryId(e.target.value)}
          disabled={saving}
        >
          <option value="">Sem categoria</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
        {bill.recurring && (
          <select
            className="txn-edit-input"
            value={paymentMethod}
            onChange={(e) => setPaymentMethod(e.target.value as BillPaymentMethod | "")}
            disabled={saving}
          >
            <option value="">Forma de pagamento</option>
            <option value="debito">Débito</option>
            <option value="credito">Crédito</option>
            <option value="pix">Pix</option>
          </select>
        )}
        {bill.recurring && (
          <span className="bill-edit-recurring">
            Repete todo mês{needsSeriesFields ? " — precisa de categoria e forma de pagamento" : ""}
          </span>
        )}
      </div>
      {error && <p className="form-error">{error}</p>}
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

function BillColumn({
  bills,
  direction,
  categories,
}: {
  bills: Bill[];
  direction: BillDirection;
  categories: Category[];
}) {
  const router = useRouter();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [seriesActionId, setSeriesActionId] = useState<string | null>(null);

  async function handleDelete(id: string) {
    if (!confirm("Excluir esta conta? Não dá para desfazer.")) return;
    setDeletingId(id);
    try {
      await deleteBillAction(id);
      router.refresh();
    } finally {
      setDeletingId(null);
    }
  }

  async function handleEndSeries(id: string) {
    setSeriesActionId(id);
    try {
      await endSeriesAction(id);
      router.refresh();
    } finally {
      setSeriesActionId(null);
    }
  }

  async function handleResumeSeries(id: string) {
    setSeriesActionId(id);
    try {
      await resumeSeriesAction(id);
      router.refresh();
    } finally {
      setSeriesActionId(null);
    }
  }

  if (bills.length === 0) {
    return <p className="empty-note">Nenhuma conta {direction === "pagar" ? "a pagar" : "a receber"}.</p>;
  }

  return (
    <div>
      {bills.map((bill) =>
        editingId === bill.id ? (
          <EditBillRow key={bill.id} bill={bill} categories={categories} onDone={() => setEditingId(null)} />
        ) : (
          <div className="bill-row" key={bill.id}>
            <div className="bill-main">
              <div className="bill-title">{bill.description}</div>
              <div className="bill-meta">
                Vence em {formatDueDate(bill.due_date)}
                {bill.recurring ? " · recorrente" : ""}
                {bill.recurring && bill.series_ended ? " · Repetição encerrada" : ""}
              </div>
            </div>
            <span className={statusPillClass(bill.status)}>{statusLabel(bill.status)}</span>
            <span className="bill-amt tab">
              {bill.amount.formatted}
              {bill.amount_estimated && <span className="bill-estimated">· estimado</span>}
            </span>
            {!bill.paid_at && (
              <form action={markBillPaidAction.bind(null, bill.id)}>
                <button className="btn-outline" type="submit">
                  {direction === "pagar" ? "Marcar como pago" : "Marcar como recebido"}
                </button>
              </form>
            )}
            <div className="row-actions">
              {bill.recurring && !bill.series_ended && (
                <button
                  className="btn-text"
                  type="button"
                  disabled={seriesActionId === bill.id}
                  onClick={() => handleEndSeries(bill.id)}
                >
                  Encerrar repetição
                </button>
              )}
              {bill.recurring && bill.series_ended && (
                <button
                  className="btn-text"
                  type="button"
                  disabled={seriesActionId === bill.id}
                  onClick={() => handleResumeSeries(bill.id)}
                >
                  Retomar repetição
                </button>
              )}
              <button className="icon-btn" type="button" aria-label="Editar" onClick={() => setEditingId(bill.id)}>
                <IconPencil />
              </button>
              <button
                className="icon-btn bad"
                type="button"
                aria-label="Excluir"
                disabled={deletingId === bill.id}
                onClick={() => handleDelete(bill.id)}
              >
                <IconTrash />
              </button>
            </div>
          </div>
        ),
      )}
    </div>
  );
}

export function BillsBoard({
  payable,
  receivable,
  categories,
}: {
  payable: Bill[];
  receivable: Bill[];
  categories: Category[];
}) {
  const [statusFilter, setStatusFilter] = useState<"" | BillStatus>("");

  const filteredPayable = useMemo(
    () => (statusFilter === "" ? payable : payable.filter((b) => b.status === statusFilter)),
    [payable, statusFilter],
  );
  const filteredReceivable = useMemo(
    () => (statusFilter === "" ? receivable : receivable.filter((b) => b.status === statusFilter)),
    [receivable, statusFilter],
  );

  return (
    <>
      <div className="list-filters">
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as "" | BillStatus)}>
          <option value="">Todos os status</option>
          <option value="pendente">Pendente</option>
          <option value="atrasado">Atrasado</option>
          <option value="pago">Pago</option>
          <option value="recebido">Recebido</option>
        </select>
      </div>

      <section className="bills-columns">
        <div className="panel">
          <div className="panel-head">
            <h2>A pagar</h2>
            <span>{filteredPayable.length} de {payable.length}</span>
          </div>
          <BillColumn bills={filteredPayable} direction="pagar" categories={categories} />
        </div>
        <div className="panel">
          <div className="panel-head">
            <h2>A receber</h2>
            <span>{filteredReceivable.length} de {receivable.length}</span>
          </div>
          <BillColumn bills={filteredReceivable} direction="receber" categories={categories} />
        </div>
      </section>
    </>
  );
}
