"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { Bill, BillDirection, BillPaymentMethod, BillStatus } from "@/lib/bills";
import type { Category, CreditCard } from "@/lib/types";
import {
  markBillPaidAction,
  updateBillAction,
  deleteBillAction,
  endSeriesAction,
  resumeSeriesAction,
  payBillAction,
  unpayBillAction,
} from "@/app/financeiro/contas/actions";
import { Modal } from "./Modal";
import { CategorySelect } from "./CategorySelect";
import { IconPencil, IconTrash } from "./icons";
import { dayKeyIn, TZ } from "@/lib/week";

// PayBillModal is what turns "marcar como pago" into a real expense: the
// bill's own amount/categoria/forma only seed the fields (the brief calls
// this out — "é o valor que realmente saiu" — since what the owner
// confirms here is what actually left the account, not the bill's estimate).
// Confirming calls payBillAction, which hits POST /api/bills/{id}/pay.
function PayBillModal({
  bill,
  categories,
  cards,
  onClose,
  onPaid,
}: {
  bill: Bill;
  categories: Category[];
  cards: CreditCard[];
  onClose: () => void;
  onPaid: () => void;
}) {
  const router = useRouter();
  const [amount, setAmount] = useState((bill.amount.cents / 100).toFixed(2).replace(".", ","));
  const [categoryId, setCategoryId] = useState(bill.category_id ?? "");
  const [paymentMethod, setPaymentMethod] = useState<BillPaymentMethod | "">(bill.payment_method ?? "");
  // A conta recorrente já pode ter um cartão salvo (a mesma que se paga todo
  // mês); só cai no primeiro da lista quando ela ainda não tem um — por
  // exemplo, a primeira vez que essa conta é paga no crédito.
  const [creditCardId, setCreditCardId] = useState(() => bill.credit_card_id ?? cards[0]?.id ?? "");
  // "hoje" é sobre onde o dono mora, não sobre o fuso do servidor/browser —
  // dayKeyIn(TZ) (frontend/lib/week.ts) já resolve isso; new Date().toISOString()
  // devolveria o dia em UTC, que vira "amanhã" das 21h às 23h59 em
  // America/Sao_Paulo.
  const [paidOn, setPaidOn] = useState(dayKeyIn(TZ));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Pay requires a category (BillService.Pay rejects an empty one), unlike
  // the create/edit form where "Sem categoria" is valid — so this dialog
  // does not offer that option, and the button stays disabled until every
  // field Pay needs is filled in.
  const canConfirm =
    amount.trim() !== "" &&
    categoryId !== "" &&
    paymentMethod !== "" &&
    (paymentMethod !== "credito" || creditCardId !== "");

  async function confirm() {
    setSaving(true);
    setError(null);
    const result = await payBillAction(bill.id, {
      amount,
      paidOn,
      categoryId,
      paymentMethod,
      creditCardId: paymentMethod === "credito" ? creditCardId : undefined,
    });
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onPaid();
  }

  return (
    <Modal open onClose={onClose}>
      <div className="panel" id="pagar-conta">
        <div className="panel-head">
          <h2>Confirmar pagamento</h2>
        </div>
        <div className="form-grid">
          <div className="field">
            <label htmlFor="p-valor">Valor</label>
            <input
              id="p-valor"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              inputMode="decimal"
              disabled={saving}
            />
          </div>
          <div className="field">
            <label htmlFor="p-cat">Categoria</label>
            <CategorySelect
              id="p-cat"
              categories={categories}
              value={categoryId}
              onChange={setCategoryId}
              disabled={saving}
            />
          </div>
          <div className="field">
            <label htmlFor="p-metodo">Forma de pagamento</label>
            <select
              id="p-metodo"
              value={paymentMethod}
              onChange={(e) => setPaymentMethod(e.target.value as BillPaymentMethod | "")}
              disabled={saving}
            >
              <option value="">Selecione</option>
              <option value="debito">Débito</option>
              <option value="credito">Crédito</option>
              <option value="pix">Pix</option>
            </select>
          </div>
          {paymentMethod === "credito" && (
            <div className="field">
              <label htmlFor="p-cartao">Cartão</label>
              <select
                id="p-cartao"
                value={creditCardId}
                onChange={(e) => setCreditCardId(e.target.value)}
                disabled={saving}
              >
                {cards.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div className="field">
            <label htmlFor="p-data">Data do pagamento</label>
            <input
              id="p-data"
              type="date"
              value={paidOn}
              onChange={(e) => setPaidOn(e.target.value)}
              disabled={saving}
            />
          </div>
        </div>
        {error && <p className="form-error">{error}</p>}
        <div className="row-actions">
          <button className="btn-text" type="button" onClick={onClose} disabled={saving}>
            Cancelar
          </button>
          <button className="btn-primary" type="button" onClick={confirm} disabled={saving || !canConfirm}>
            {saving ? "Salvando…" : "Confirmar pagamento"}
          </button>
        </div>
      </div>
    </Modal>
  );
}

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

function EditBillRow({
  bill,
  categories,
  cards,
  onDone,
}: {
  bill: Bill;
  categories: Category[];
  cards: CreditCard[];
  onDone: () => void;
}) {
  const router = useRouter();
  const [description, setDescription] = useState(bill.description);
  const [amount, setAmount] = useState((bill.amount.cents / 100).toFixed(2).replace(".", ","));
  const [dueDate, setDueDate] = useState(bill.due_date);
  const [categoryId, setCategoryId] = useState(bill.category_id ?? "");
  const [paymentMethod, setPaymentMethod] = useState<BillPaymentMethod | "">(bill.payment_method ?? "");
  const [creditCardId, setCreditCardId] = useState(() => bill.credit_card_id ?? cards[0]?.id ?? "");
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
      credit_card_id: paymentMethod === "credito" ? creditCardId : undefined,
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
        <CategorySelect
          className="txn-edit-input"
          categories={categories}
          value={categoryId}
          onChange={setCategoryId}
          disabled={saving}
          emptyLabel="Sem categoria"
          emptySelectable
        />
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
        {bill.recurring && paymentMethod === "credito" && (
          <select
            className="txn-edit-input"
            value={creditCardId}
            onChange={(e) => setCreditCardId(e.target.value)}
            disabled={saving}
          >
            {cards.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
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
  cards,
}: {
  bills: Bill[];
  direction: BillDirection;
  categories: Category[];
  cards: CreditCard[];
}) {
  const router = useRouter();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [seriesActionId, setSeriesActionId] = useState<string | null>(null);
  const [endingBill, setEndingBill] = useState<Bill | null>(null);
  const [payingBill, setPayingBill] = useState<Bill | null>(null);
  const [unpayingId, setUnpayingId] = useState<string | null>(null);
  const [unpayError, setUnpayError] = useState<{ id: string; message: string } | null>(null);

  async function handleDelete(bill: Bill) {
    // A recurring, still-active series' occurrence may be the LATEST one —
    // only the backend can tell for sure (see BillService.Delete) — in which
    // case deleting it also ends the series, or the very next month-open
    // would materialize it right back from the occurrence before it, undoing
    // the delete and any correction made to that row. The confirmation says
    // so honestly instead of the old "não dá para desfazer", which was
    // actively wrong for exactly this case. A one-off or already-ended
    // series keeps the plain wording: nothing else is at stake for those.
    const message =
      bill.recurring && !bill.series_ended
        ? "Excluir esta conta e encerrar a repetição? Os meses já registrados continuam no histórico."
        : "Excluir esta conta? Não dá para desfazer.";
    if (!confirm(message)) return;
    setDeletingId(bill.id);
    try {
      await deleteBillAction(bill.id);
      router.refresh();
    } finally {
      setDeletingId(null);
    }
  }

  async function handleUnpay(id: string) {
    if (!confirm("Desfazer o pagamento? O lançamento criado por ele também será apagado.")) return;
    setUnpayingId(id);
    setUnpayError(null);
    try {
      const result = await unpayBillAction(id);
      if (result.error) {
        setUnpayError({ id, message: result.error });
        return;
      }
      router.refresh();
    } finally {
      setUnpayingId(null);
    }
  }

  async function handleEndSeries(id: string, deleteFollowing: boolean) {
    setSeriesActionId(id);
    try {
      await endSeriesAction(id, deleteFollowing);
      setEndingBill(null);
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
          <EditBillRow key={bill.id} bill={bill} categories={categories} cards={cards} onDone={() => setEditingId(null)} />
        ) : (
          <div className="bill-row" key={bill.id}>
            <div className="bill-main">
              <div className="bill-title">{bill.description}</div>
              <div className="bill-meta">
                Vence em {formatDueDate(bill.due_date)}
                {bill.invoice_card_id ? " · soma das compras no cartão" : ""}
                {bill.recurring ? " · recorrente" : ""}
                {bill.recurring && bill.series_ended ? " · Repetição encerrada" : ""}
              </div>
              {unpayError?.id === bill.id && <p className="form-error">{unpayError.message}</p>}
            </div>
            <span className={statusPillClass(bill.status)}>{statusLabel(bill.status)}</span>
            <span className="bill-amt tab">
              {bill.amount.formatted}
              {bill.amount_estimated && <span className="bill-estimated">· estimado</span>}
            </span>
            {!bill.paid_at && bill.invoice_card_id && (
              <form action={markBillPaidAction.bind(null, bill.id)}>
                <button className="btn-outline" type="submit">
                  Marcar como paga
                </button>
              </form>
            )}
            {!bill.paid_at && direction === "pagar" && !bill.invoice_card_id && (
              <button className="btn-outline" type="button" onClick={() => setPayingBill(bill)}>
                Marcar como pago
              </button>
            )}
            {!bill.paid_at && direction === "receber" && (
              <form action={markBillPaidAction.bind(null, bill.id)}>
                <button className="btn-outline" type="submit">
                  Marcar como recebido
                </button>
              </form>
            )}
            <div className="row-actions">
              {bill.paid_at && (
                <button
                  className="btn-text"
                  type="button"
                  disabled={unpayingId === bill.id}
                  onClick={() => handleUnpay(bill.id)}
                >
                  Desfazer pagamento
                </button>
              )}
              {bill.recurring && !bill.series_ended && (
                <button
                  className="btn-text"
                  type="button"
                  disabled={seriesActionId === bill.id}
                  onClick={() => setEndingBill(bill)}
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
              {!bill.invoice_card_id && (
                <>
                  <button className="icon-btn" type="button" aria-label="Editar" onClick={() => setEditingId(bill.id)}>
                    <IconPencil />
                  </button>
                  <button
                    className="icon-btn bad"
                    type="button"
                    aria-label="Excluir"
                    disabled={deletingId === bill.id}
                    onClick={() => handleDelete(bill)}
                  >
                    <IconTrash />
                  </button>
                </>
              )}
            </div>
          </div>
        ),
      )}
      {payingBill && (
        <PayBillModal
          bill={payingBill}
          categories={categories}
          cards={cards}
          onClose={() => setPayingBill(null)}
          onPaid={() => setPayingBill(null)}
        />
      )}
      {endingBill && (
        <Modal open onClose={() => setEndingBill(null)}>
          <div className="panel">
            <div className="panel-head">
              <h2>Encerrar repetição</h2>
            </div>
            <p className="empty-note">
              {endingBill.description} para de se repetir depois de {formatDueDate(endingBill.due_date)}. As contas já pagas
              ficam no histórico.
            </p>
            <div className="form-grid">
              <button
                className="btn-block"
                type="button"
                disabled={seriesActionId === endingBill.id}
                onClick={() => handleEndSeries(endingBill.id, false)}
              >
                Só encerrar
              </button>
              <button
                className="btn-outline bad"
                type="button"
                disabled={seriesActionId === endingBill.id}
                onClick={() => handleEndSeries(endingBill.id, true)}
              >
                Encerrar e excluir as seguintes não pagas
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

export function BillsBoard({
  payable,
  receivable,
  categories,
  cards,
}: {
  payable: Bill[];
  receivable: Bill[];
  categories: Category[];
  cards: CreditCard[];
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
          <BillColumn bills={filteredPayable} direction="pagar" categories={categories} cards={cards} />
        </div>
        <div className="panel">
          <div className="panel-head">
            <h2>A receber</h2>
            <span>{filteredReceivable.length} de {receivable.length}</span>
          </div>
          <BillColumn bills={filteredReceivable} direction="receber" categories={categories} cards={cards} />
        </div>
      </section>
    </>
  );
}
