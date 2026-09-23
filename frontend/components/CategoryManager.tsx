"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Category } from "@/lib/types";
import { createCategoryAction, updateCategoryAction, deleteCategoryAction } from "@/app/financeiro/actions";
import { Modal } from "./Modal";
import { IconPencil, IconTrash } from "./icons";

const NATURE_LABEL: Record<Category["nature"], string> = {
  essencial: "Essencial",
  variavel: "Variável",
  investimento: "Investimento",
};

// budget is what was typed, not cents: the action parses it, so "1.200,50"
// and "" (no budget) both survive the trip unchanged.
type FormState = { name: string; kind: Category["kind"]; nature: Category["nature"]; color: string; budget: string };

// budgetOf pre-fills the field with the amount already set, in the plain
// "1200,50" the field expects — never the formatted "R$ 1.200,50", which
// would come back as a different number if saved untouched.
function budgetOf(category: Category): string {
  const cents = category.monthly_budget?.cents;
  return cents ? (cents / 100).toFixed(2).replace(".", ",") : "";
}

function CategoryForm({
  initial,
  onSaved,
  onCancel,
  save,
}: {
  initial: FormState;
  onSaved: () => void;
  onCancel?: () => void;
  save: (input: FormState) => Promise<{ error?: string }>;
}) {
  const router = useRouter();
  const [form, setForm] = useState<FormState>(initial);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSave() {
    setSaving(true);
    setError(null);
    const result = await save(form);
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onSaved();
  }

  return (
    <div className="form-grid">
      <div className="field">
        <label htmlFor="cat-name">Nome</label>
        <input
          id="cat-name"
          type="text"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          disabled={saving}
        />
      </div>
      <div className="field">
        <label>Tipo</label>
        <div className="seg">
          <button
            type="button"
            className={form.kind === "despesa" ? "active" : ""}
            onClick={() => setForm({ ...form, kind: "despesa" })}
            disabled={saving}
          >
            Despesa
          </button>
          <button
            type="button"
            className={form.kind === "receita" ? "active" : ""}
            onClick={() => setForm({ ...form, kind: "receita" })}
            disabled={saving}
          >
            Receita
          </button>
        </div>
      </div>
      <div className="row2">
        <div className="field">
          <label htmlFor="cat-nature">Natureza</label>
          <select
            id="cat-nature"
            value={form.nature}
            onChange={(e) => setForm({ ...form, nature: e.target.value as Category["nature"] })}
            disabled={saving}
          >
            <option value="essencial">Essencial</option>
            <option value="variavel">Variável</option>
            <option value="investimento">Investimento</option>
          </select>
        </div>
        <div className="field">
          <label htmlFor="cat-color">Cor</label>
          <input
            id="cat-color"
            type="color"
            className="color-input"
            value={form.color}
            onChange={(e) => setForm({ ...form, color: e.target.value })}
            disabled={saving}
          />
        </div>
      </div>
      <div className="field">
        <label htmlFor="cat-budget">Orçamento mensal</label>
        <input
          id="cat-budget"
          type="text"
          inputMode="decimal"
          placeholder="Deixe em branco para não ter orçamento"
          value={form.budget}
          onChange={(e) => setForm({ ...form, budget: e.target.value })}
          disabled={saving}
        />
      </div>
      {error && <p className="form-error">{error}</p>}
      <div className="row-actions" style={{ justifyContent: "flex-end" }}>
        {onCancel && (
          <button className="btn-text" type="button" onClick={onCancel} disabled={saving}>
            Cancelar
          </button>
        )}
        <button className="btn-block" type="button" onClick={handleSave} disabled={saving || !form.name.trim()}>
          {saving ? "Salvando…" : "Salvar"}
        </button>
      </div>
    </div>
  );
}

function CategoryRow({ category }: { category: Category }) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  async function handleDelete() {
    if (!confirm(`Excluir a categoria "${category.name}"? Só é possível se não houver lançamentos ou contas usando ela.`))
      return;
    setDeleting(true);
    setDeleteError(null);
    const result = await deleteCategoryAction(category.id);
    setDeleting(false);
    if (result.error) {
      setDeleteError(result.error);
      return;
    }
    router.refresh();
  }

  if (editing) {
    return (
      <div className="category-manager-row category-manager-row-editing">
        <CategoryForm
          initial={{
            name: category.name,
            kind: category.kind,
            nature: category.nature,
            color: category.color,
            budget: budgetOf(category),
          }}
          onSaved={() => setEditing(false)}
          onCancel={() => setEditing(false)}
          save={({ budget, ...input }) => updateCategoryAction(category.id, input, budget)}
        />
      </div>
    );
  }

  return (
    <div className="category-manager-row">
      <span className="dot" style={{ background: category.color }} />
      <span className="category-manager-name">{category.name}</span>
      <span className="category-manager-nature">
        {category.kind === "receita" ? "Receita" : "Despesa"} · {NATURE_LABEL[category.nature]}
      </span>
      <span className="category-manager-budget">
        {category.monthly_budget ? category.monthly_budget.formatted : "sem orçamento"}
      </span>
      <div className="row-actions">
        <button className="icon-btn" type="button" aria-label="Editar categoria" onClick={() => setEditing(true)}>
          <IconPencil />
        </button>
        <button
          className="icon-btn bad"
          type="button"
          aria-label="Excluir categoria"
          disabled={deleting}
          onClick={handleDelete}
        >
          <IconTrash />
        </button>
      </div>
      {deleteError && <p className="form-error category-manager-error">{deleteError}</p>}
    </div>
  );
}

const RANDOM_COLORS = ["#2a78d6", "#eb6834", "#1baf7a", "#c98d00", "#e2588e", "#008300", "#4a3aa7"];

export function CategoryManager({ categories }: { categories: Category[] }) {
  const [creating, setCreating] = useState(false);
  const nextColor = RANDOM_COLORS[categories.length % RANDOM_COLORS.length];

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Categorias</h2>
        <button className="btn-text" type="button" onClick={() => setCreating(true)}>
          + Nova categoria
        </button>
      </div>

      {categories.length === 0 ? (
        <p className="empty-note">Nenhuma categoria ainda.</p>
      ) : (
        categories.map((c) => <CategoryRow key={c.id} category={c} />)
      )}

      <Modal open={creating} onClose={() => setCreating(false)}>
        <div className="panel">
          <div className="panel-head">
            <h2>Nova categoria</h2>
          </div>
          <CategoryForm
            initial={{ name: "", kind: "despesa", nature: "variavel", color: nextColor, budget: "" }}
            onSaved={() => setCreating(false)}
            save={({ budget, ...input }) => createCategoryAction(input, budget)}
          />
        </div>
      </Modal>
    </div>
  );
}
