"use client";

import { useActionState, useEffect, useMemo, useState } from "react";
import { createTransactionAction, type CreateTransactionState } from "@/app/financeiro/lancamentos/actions";
import type { Category, CreditCard, PaymentMethod } from "@/lib/types";
import { creditCardCompetenceYearMonth, formatYearMonth } from "@/lib/month";

const initialState: CreateTransactionState = { status: "idle" };

export function NewTransactionForm({
  categories,
  cards,
  onSuccess,
}: {
  categories: Category[];
  cards: CreditCard[];
  onSuccess?: () => void;
}) {
  const [state, formAction, pending] = useActionState(createTransactionAction, initialState);

  useEffect(() => {
    if (state.status === "success") onSuccess?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);
  const [method, setMethod] = useState<PaymentMethod>("debito");
  const [installments, setInstallments] = useState(1);
  const [purchaseDate, setPurchaseDate] = useState(() => new Date().toISOString().slice(0, 10));
  const [recurring, setRecurring] = useState(false);
  const [cardId, setCardId] = useState(() => cards[0]?.id ?? "");

  // The competence month depends on the card's closing/due days (see
  // creditCardCompetenceYearMonth), not on a fixed offset — so the select
  // has to be controlled for this hint to react to which card is chosen.
  const { competenceYearMonth, purchaseYearMonth } = useMemo(() => {
    if (!purchaseDate) return { competenceYearMonth: null, purchaseYearMonth: null };
    const [y, m] = purchaseDate.split("-").map(Number);
    const pym = `${y}-${String(m).padStart(2, "0")}`;
    if (method !== "credito") return { competenceYearMonth: pym, purchaseYearMonth: pym };
    const card = cards.find((c) => c.id === cardId);
    if (!card) return { competenceYearMonth: null, purchaseYearMonth: pym };
    return { competenceYearMonth: creditCardCompetenceYearMonth(purchaseDate, card), purchaseYearMonth: pym };
  }, [purchaseDate, method, cardId, cards]);

  const competenceHint = competenceYearMonth ? formatYearMonth(competenceYearMonth) : null;
  const competenceIsPurchaseMonth = competenceYearMonth === purchaseYearMonth;

  return (
    <div className="panel" id="novo-lancamento">
      <div className="panel-head">
        <h2>Novo lançamento</h2>
      </div>
      <form action={formAction} className="form-grid">
        <div className="field">
          <label htmlFor="f-desc">Descrição</label>
          <input id="f-desc" name="description" type="text" placeholder="Ex: Supermercado" required />
        </div>

        <div className="row2">
          <div className="field">
            <label htmlFor="f-valor">Valor</label>
            <input id="f-valor" name="amount" type="text" inputMode="decimal" placeholder="0,00" required />
          </div>
          <div className="field">
            <label htmlFor="f-cat">Categoria</label>
            <select id="f-cat" name="category_id" required defaultValue="">
              <option value="" disabled>
                Selecione
              </option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
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
          </>
        )}

        {competenceHint && (
          <p className="helper">
            {installments > 1 ? (
              <>
                Primeira parcela entra na fatura de <strong>{competenceHint}</strong>; as demais nos meses seguintes.
              </>
            ) : (
              <>
                {method === "credito" ? "Essa compra" : "Esse lançamento"} entra no gasto de{" "}
                <strong>{competenceHint}</strong>
                {method === "credito" && !competenceIsPurchaseMonth ? ", não no mês da compra" : ""}.
              </>
            )}
          </p>
        )}

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

        <button className="btn-block" type="submit" disabled={pending}>
          {pending ? "Salvando…" : "Salvar lançamento"}
        </button>

        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}
