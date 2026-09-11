"use client";

import { useActionState, useEffect, useState } from "react";
import { createBillAction, type BillFormState } from "@/app/financeiro/contas/actions";
import type { Category } from "@/lib/types";
import type { BillDirection } from "@/lib/bills";

const initialState: BillFormState = { status: "idle" };

export function BillForm({ categories, onSuccess }: { categories: Category[]; onSuccess?: () => void }) {
  const [state, formAction, pending] = useActionState(createBillAction, initialState);
  const [direction, setDirection] = useState<BillDirection>("pagar");
  const [recurring, setRecurring] = useState(false);

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
          <label htmlFor="b-cat">Categoria (opcional)</label>
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

        <button className="btn-block" type="submit" disabled={pending}>
          {pending ? "Salvando…" : "Salvar conta"}
        </button>

        {state.status === "error" && <p className="form-error">{state.message}</p>}
        {state.status === "success" && <p className="form-success">{state.message}</p>}
      </form>
    </div>
  );
}
