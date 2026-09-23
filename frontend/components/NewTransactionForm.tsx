"use client";

import { useActionState, useEffect, useMemo, useState } from "react";
import {
  createTransactionAction,
  replaceTransactionAction,
  type CreateTransactionState,
} from "@/app/financeiro/lancamentos/actions";
import { CategorySelect } from "./CategorySelect";
import type { Category, CreditCard, PaymentMethod, Transaction } from "@/lib/types";
import { creditCardInvoiceYearMonth, formatYearMonth } from "@/lib/month";
import { dayKeyIn, TZ } from "@/lib/week";

const initialState: CreateTransactionState = { status: "idle" };

const brl = (cents: number) => (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

// Mirrors the action's parser: "189,90" or "1.234,56".
function parseAmountToCents(raw: string): number | null {
  const value = Number.parseFloat(raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", "."));
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}
function centsToText(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// With `editing`, the form starts from that purchase and saving rewrites it
// whole — every installment, card and date included.
export function NewTransactionForm({
  categories,
  cards,
  onSuccess,
  editing,
}: {
  categories: Category[];
  cards: CreditCard[];
  onSuccess?: () => void;
  editing?: Transaction;
}) {
  const [state, formAction, pending] = useActionState(
    editing ? replaceTransactionAction.bind(null, editing.id) : createTransactionAction,
    initialState,
  );

  useEffect(() => {
    if (state.status === "success") onSuccess?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);
  const [method, setMethod] = useState<PaymentMethod>(editing?.payment_method ?? "debito");
  const [installments, setInstallments] = useState(editing?.installment_total ?? 1);
  // dayKeyIn(TZ), não toISOString(): este último devolve o dia em UTC, e o
  // dono está em UTC-3 — entre 21h e meia-noite ele daria AMANHÃ como hoje,
  // datando a compra no mês errado sem ninguém notar.
  const [purchaseDate, setPurchaseDate] = useState(() => editing?.purchase_date ?? dayKeyIn(TZ));
  const [recurring, setRecurring] = useState(editing?.is_recurring ?? false);
  const [amountText, setAmountText] = useState(editing ? centsToText(editing.purchase_total.cents) : "");
  // With installments, the amount typed can be the whole purchase or one
  // installment; the action turns the latter into the total.
  const [amountMode, setAmountMode] = useState<"total" | "parcela">("total");
  const parceled = method === "credito" && installments > 1;
  const [cardId, setCardId] = useState(() => editing?.credit_card_id ?? cards[0]?.id ?? "");
  const [categoryId, setCategoryId] = useState(editing?.category_id ?? "");
  const [creatingCategory, setCreatingCategory] = useState(false);

  // The purchase shows up in the month it was made; on credit it is also
  // billed on an invoice that depends on the card's closing/due days, so the
  // card select is controlled for this hint to follow it.
  const { purchaseYearMonth, invoiceYearMonth } = useMemo(() => {
    if (!purchaseDate) return { purchaseYearMonth: null, invoiceYearMonth: null };
    const [y, m] = purchaseDate.split("-").map(Number);
    const pym = `${y}-${String(m).padStart(2, "0")}`;
    const card = method === "credito" ? cards.find((c) => c.id === cardId) : undefined;
    return { purchaseYearMonth: pym, invoiceYearMonth: card ? creditCardInvoiceYearMonth(purchaseDate, card) : null };
  }, [purchaseDate, method, cardId, cards]);

  const installmentHint = useMemo(() => {
    if (!parceled) return null;
    const cents = parseAmountToCents(amountText);
    if (cents === null) return null;
    const total = amountMode === "total" ? cents : cents * installments;
    // Same split as the server: equal parts, the remainder on the last one.
    const part = Math.floor(total / installments);
    const last = total - part * (installments - 1);
    const each = last === part ? `${installments}x de ${brl(part)}` : `${installments - 1}x de ${brl(part)} + 1x de ${brl(last)}`;
    return `${each} · total ${brl(total)}`;
  }, [parceled, amountText, amountMode, installments]);

  const purchaseHint = purchaseYearMonth ? formatYearMonth(purchaseYearMonth) : null;
  const invoiceHint = invoiceYearMonth ? formatYearMonth(invoiceYearMonth) : null;

  return (
    <div className="panel" id="novo-lancamento">
      <div className="panel-head">
        <h2>{editing ? "Editar lançamento" : "Novo lançamento"}</h2>
      </div>
      <form action={formAction} className="form-grid">
        <div className="field">
          <label htmlFor="f-desc">Descrição</label>
          <input
            id="f-desc"
            name="description"
            type="text"
            placeholder="Ex: Supermercado"
            required
            defaultValue={editing?.description}
          />
        </div>

        <div className="row2">
          <div className="field">
            <label htmlFor="f-valor">{parceled ? (amountMode === "total" ? "Valor total" : "Valor da parcela") : "Valor"}</label>
            <input
              id="f-valor"
              name="amount"
              type="text"
              inputMode="decimal"
              placeholder="0,00"
              required
              value={amountText}
              onChange={(e) => setAmountText(e.target.value)}
            />
            <input type="hidden" name="amount_mode" value={parceled ? amountMode : "total"} />
          </div>
          <div className="field">
            <label htmlFor="f-cat">Categoria</label>
            <CategorySelect
              id="f-cat"
              name="category_id"
              required
              categories={categories}
              value={categoryId}
              onChange={setCategoryId}
              onCreatingChange={setCreatingCategory}
            />
          </div>
        </div>

        <div className="field">
          <label htmlFor="f-data">Data da compra</label>
          <input
            id="f-data"
            name="purchase_date"
            type="date"
            required
            value={purchaseDate}
            onChange={(e) => setPurchaseDate(e.target.value)}
          />
        </div>

        <div className="field">
          <label>Forma de pagamento</label>
          <input type="hidden" name="payment_method" value={method} />
          <div className="seg">
            {(["debito", "credito", "pix"] as const).map((m) => (
              <button
                key={m}
                type="button"
                className={method === m ? "active" : ""}
                onClick={() => setMethod(m)}
              >
                {m === "debito" ? "Débito" : m === "credito" ? "Crédito" : "Pix"}
              </button>
            ))}
          </div>
        </div>

        {method === "credito" && (
          <>
            <div className="field">
              <label htmlFor="f-card">Cartão</label>
              <select
                id="f-card"
                name="credit_card_id"
                required
                value={cardId}
                onChange={(e) => setCardId(e.target.value)}
              >
                {cards.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="field">
              <label>Parcelas</label>
              <input type="hidden" name="installments" value={installments} />
              <div className="stepper">
                <button type="button" aria-label="Diminuir" onClick={() => setInstallments((n) => Math.max(1, n - 1))}>
                  –
                </button>
                <span className="val tab">{installments}</span>
                <button type="button" aria-label="Aumentar" onClick={() => setInstallments((n) => Math.min(24, n + 1))}>
                  +
                </button>
              </div>
            </div>

            {installments > 1 && (
              <div className="field">
                <label>O valor digitado é</label>
                <div className="seg">
                  <button type="button" className={amountMode === "total" ? "active" : ""} onClick={() => setAmountMode("total")}>
                    Total da compra
                  </button>
                  <button
                    type="button"
                    className={amountMode === "parcela" ? "active" : ""}
                    onClick={() => setAmountMode("parcela")}
                  >
                    Cada parcela
                  </button>
                </div>
              </div>
            )}
          </>
        )}

        {purchaseHint && (
          <p className="helper">
            {installments > 1 && invoiceHint ? (
              <>
                {installmentHint && (
                  <>
                    <strong>{installmentHint}</strong>.{" "}
                  </>
                )}
                As parcelas aparecem de <strong>{purchaseHint}</strong> em diante. A primeira entra na fatura de{" "}
                <strong>{invoiceHint}</strong>.
              </>
            ) : (
              <>
                {method === "credito" ? "Essa compra" : "Esse lançamento"} aparece em <strong>{purchaseHint}</strong>
                {invoiceHint ? (
                  <>
                    {" "}
                    e entra na fatura de <strong>{invoiceHint}</strong>
                  </>
                ) : null}
                .
              </>
            )}
          </p>
        )}

        {/* A parceled purchase already spreads over the months; repeating it
            would add a whole new purchase every month. */}
        {!parceled && (
          <div className="field toggle-row">
            <label style={{ margin: 0 }} htmlFor="f-recurring">
              Repetir todo mês
            </label>
            <input type="hidden" name="is_recurring" value={recurring ? "1" : ""} />
            <button
              id="f-recurring"
              type="button"
              className={`switch ${recurring ? "on" : ""}`}
              role="switch"
              aria-checked={recurring}
              onClick={() => setRecurring((v) => !v)}
            />
          </div>
        )}

        <button className="btn-block" type="submit" disabled={pending || creatingCategory}>
          {pending ? "Salvando…" : editing ? "Salvar alterações" : "Salvar lançamento"}
        </button>

        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}
