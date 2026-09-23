"use client";

import { useActionState, useEffect, useState } from "react";
import { createBillAction, type BillFormState } from "@/app/financeiro/contas/actions";
import type { Category, CreditCard } from "@/lib/types";
import type { BillDirection, BillPaymentMethod } from "@/lib/bills";
import { CategorySelect } from "./CategorySelect";

const initialState: BillFormState = { status: "idle" };

export function BillForm({
  categories,
  cards,
  onSuccess,
}: {
  categories: Category[];
  cards: CreditCard[];
  onSuccess?: () => void;
}) {
  const [state, formAction, pending] = useActionState(createBillAction, initialState);
  const [direction, setDirection] = useState<BillDirection>("pagar");
  const [categoryId, setCategoryId] = useState("");
  const [recurring, setRecurring] = useState(false);
  const [amountVaries, setAmountVaries] = useState(false);
  const [paymentMethod, setPaymentMethod] = useState<BillPaymentMethod | "">("");
  const [creditCardId, setCreditCardId] = useState(() => cards[0]?.id ?? "");

  // A conta que se repete só vira gasto sozinha quando quitada se souber a
  // categoria e a forma de pagamento; sem isso o backend recusa com 422.
  const needsSeriesFields = recurring && direction === "pagar";

  useEffect(() => {
    if (state.status === "success") onSuccess?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.status]);

  return (
    <div className="panel" id="nova-conta">
      <div className="panel-head">
        <h2>Nova conta</h2>
      </div>
      <form action={formAction} className="form-grid">
        <div className="field">
          <label htmlFor="b-desc">Descrição</label>
          <input id="b-desc" name="description" type="text" placeholder="Ex: Conta de luz" required />
        </div>

        <div className="row2">
          <div className="field">
            <label htmlFor="b-valor">Valor</label>
            <input id="b-valor" name="amount" type="text" inputMode="decimal" placeholder="0,00" required />
          </div>
          <div className="field">
            <label htmlFor="b-venc">Vencimento</label>
            <input id="b-venc" name="due_date" type="date" required />
          </div>
        </div>

        <div className="field">
          <label>Direção</label>
          <input type="hidden" name="direction" value={direction} />
          <div className="seg">
            <button type="button" className={direction === "pagar" ? "active" : ""} onClick={() => setDirection("pagar")}>
              A pagar
            </button>
            <button type="button" className={direction === "receber" ? "active" : ""} onClick={() => setDirection("receber")}>
              A receber
            </button>
          </div>
        </div>

        <div className="field">
          <label htmlFor="b-cat">Categoria{needsSeriesFields ? "" : " (opcional)"}</label>
          <CategorySelect
            id="b-cat"
            name="category_id"
            categories={categories}
            value={categoryId}
            onChange={setCategoryId}
            emptyLabel="Sem categoria"
            emptySelectable
          />
        </div>

        <div className="field toggle-row">
          <label style={{ margin: 0 }} htmlFor="b-recurring">
            Repetir todo mês
          </label>
          <input type="hidden" name="recurring" value={recurring ? "1" : ""} />
          <button
            id="b-recurring"
            type="button"
            className={`switch ${recurring ? "on" : ""}`}
            role="switch"
            aria-checked={recurring}
            onClick={() => setRecurring((v) => !v)}
          />
        </div>

        {recurring && (
          <>
            <div className="field toggle-row">
              <label style={{ margin: 0 }} htmlFor="b-varies">
                O valor muda todo mês
              </label>
              <input type="hidden" name="amount_varies" value={amountVaries ? "1" : ""} />
              <button
                id="b-varies"
                type="button"
                className={`switch ${amountVaries ? "on" : ""}`}
                role="switch"
                aria-checked={amountVaries}
                onClick={() => setAmountVaries((v) => !v)}
              />
            </div>
            {amountVaries && (
              <p className="helper">
                A conta do mês seguinte nasce com o valor deste mês, para você corrigir quando ela chegar.
              </p>
            )}

            <div className="field">
              <label htmlFor="b-payment">Forma de pagamento{direction === "pagar" ? "" : " (opcional)"}</label>
              <select
                id="b-payment"
                name="payment_method"
                value={paymentMethod}
                onChange={(e) => setPaymentMethod(e.target.value as BillPaymentMethod | "")}
              >
                <option value="">Selecione</option>
                <option value="debito">Débito</option>
                <option value="credito">Crédito</option>
                <option value="pix">Pix</option>
              </select>
            </div>

            {paymentMethod === "credito" && (
              <div className="field">
                <label htmlFor="b-cartao">Cartão</label>
                <select
                  id="b-cartao"
                  name="credit_card_id"
                  value={creditCardId}
                  onChange={(e) => setCreditCardId(e.target.value)}
                  required
                >
                  {cards.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </>
        )}

        <button className="btn-block" type="submit" disabled={pending}>
          {pending ? "Salvando…" : "Salvar conta"}
        </button>

        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}
