"use client";

import { useActionState, useEffect, useState } from "react";
import { createBillAction, type BillFormState } from "@/app/financeiro/contas/actions";
import type { Category } from "@/lib/types";
import type { BillDirection, BillPaymentMethod } from "@/lib/bills";

const initialState: BillFormState = { status: "idle" };

export function BillForm({ categories, onSuccess }: { categories: Category[]; onSuccess?: () => void }) {
  const [state, formAction, pending] = useActionState(createBillAction, initialState);
  const [direction, setDirection] = useState<BillDirection>("pagar");
  const [recurring, setRecurring] = useState(false);
  const [amountEstimated, setAmountEstimated] = useState(false);
  const [paymentMethod, setPaymentMethod] = useState<BillPaymentMethod | "">("");

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
          <select id="b-cat" name="category_id" defaultValue="">
            <option value="">Sem categoria</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
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
              <label style={{ margin: 0 }} htmlFor="b-estimated">
                O valor muda todo mês
              </label>
              <input type="hidden" name="amount_estimated" value={amountEstimated ? "1" : ""} />
              <button
                id="b-estimated"
                type="button"
                className={`switch ${amountEstimated ? "on" : ""}`}
                role="switch"
                aria-checked={amountEstimated}
                onClick={() => setAmountEstimated((v) => !v)}
              />
            </div>
            {amountEstimated && (
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
