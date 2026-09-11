"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { CategorySlice } from "@/lib/types";
import { setCategoryBudgetAction } from "@/app/financeiro/actions";
import { IconPencil } from "./icons";

function buildConicGradient(categories: CategorySlice[], totalCents: number): string {
  if (totalCents <= 0) return "var(--surface-2)";
  let acc = 0;
  const stops = categories
    .filter((c) => c.total.cents > 0)
    .map((c) => {
      const start = (acc / totalCents) * 100;
      acc += c.total.cents;
      const end = (acc / totalCents) * 100;
      return `${c.color} ${start}% ${end}%`;
    });
  return `conic-gradient(${stops.join(", ")})`;
}

function BudgetEditor({ categoryId, current, onDone }: { categoryId: string; current?: number; onDone: () => void }) {
  const router = useRouter();
  const [value, setValue] = useState(current ? (current / 100).toFixed(2).replace(".", ",") : "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    const result = await setCategoryBudgetAction(categoryId, value);
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onDone();
  }

  return (
    <div className="legend-budget-editor">
      <input
        className="txn-edit-input"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Sem orçamento"
        inputMode="decimal"
        disabled={saving}
      />
      <button className="btn-text" type="button" onClick={onDone} disabled={saving}>
        Cancelar
      </button>
      <button className="btn-text" type="button" onClick={save} disabled={saving}>
        {saving ? "Salvando…" : "Salvar"}
      </button>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}

export function CategoryPieChart({ categories }: { categories: CategorySlice[] }) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const totalCents = useMemo(() => categories.reduce((sum, c) => sum + c.total.cents, 0), [categories]);
  const gradient = useMemo(() => buildConicGradient(categories, totalCents), [categories, totalCents]);
  const withSpend = categories.filter((c) => c.total.cents > 0 || c.monthly_budget);

  return (
    <div className="panel donut-panel">
      <div className="panel-head">
        <h2>Gasto por categoria</h2>
        <span>este mês</span>
      </div>

      <div className="donut-body">
        <div className="donut" style={{ background: gradient }}>
          <div className="donut-center">
            <span className="donut-center-label">Total</span>
            <span className="donut-center-value tab">
              {(totalCents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}
            </span>
          </div>
        </div>

        <div className="category-legend">
          {withSpend.length === 0 ? (
            <p className="empty-note">Nenhum gasto lançado neste mês ainda.</p>
          ) : (
            withSpend.map((c) => {
              const budget = c.monthly_budget?.cents;
              const pct = budget ? Math.min((c.total.cents / budget) * 100, 100) : null;
              const over = budget ? c.total.cents > budget : false;
              return (
                <div className="legend-row" key={c.category_id}>
                  <div className="legend-row-main">
                    <div className="legend-name">
                      <span className="dot" style={{ background: c.color }} />
                      {c.name}
                      <button
                        className="icon-btn legend-edit-btn"
                        type="button"
                        aria-label="Definir orçamento"
                        onClick={() => setEditingId(c.category_id)}
                      >
                        <IconPencil />
                      </button>
                    </div>
                    {editingId === c.category_id ? (
                      <BudgetEditor
                        categoryId={c.category_id}
                        current={c.monthly_budget?.cents}
                        onDone={() => setEditingId(null)}
                      />
                    ) : (
                      budget && (
                        <div className="legend-budget-bar-track">
                          <div
                            className="legend-budget-bar-fill"
                            style={{ width: `${pct}%`, background: over ? "var(--bad)" : c.color }}
                          />
                        </div>
                      )
                    )}
                    {budget && editingId !== c.category_id && (
                      <span className={`legend-budget-text ${over ? "legend-budget-over" : ""}`}>
                        {c.total.formatted} de {c.monthly_budget!.formatted}
                        {over ? " · acima do orçamento" : ""}
                      </span>
                    )}
                  </div>
                  {!budget && editingId !== c.category_id && <span className="legend-amt tab">{c.total.formatted}</span>}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
