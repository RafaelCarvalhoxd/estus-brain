"use client";

import { useState } from "react";
import { createCategoryAction } from "@/app/financeiro/actions";
import type { Category } from "@/lib/types";

const NEW_CATEGORY = "__nova__";
const CATEGORY_COLORS = ["#2a78d6", "#eb6834", "#1baf7a", "#c98d00", "#e2588e", "#008300", "#4a3aa7"];

// Creates a category without leaving the form it sits in. It is a div, not
// a form: forms can't nest, so Enter is caught here instead of submitting
// the outer form.
function InlineCategoryForm({
  color: initialColor,
  onCreated,
  onCancel,
}: {
  color: string;
  onCreated: (c: Category) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [nature, setNature] = useState<Category["nature"]>("variavel");
  const [kind, setKind] = useState<Category["kind"]>("despesa");
  const [color, setColor] = useState(initialColor);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create() {
    setSaving(true);
    setError(null);
    const result = await createCategoryAction({ name: name.trim(), kind, nature, color });
    setSaving(false);
    if (result.error || !result.category) {
      setError(result.error ?? "Falha ao criar a categoria.");
      return;
    }
    onCreated(result.category);
  }

  return (
    <div className="inline-category">
      <input
        type="text"
        aria-label="Nome da nova categoria"
        placeholder="Nome da categoria"
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            if (name.trim()) create();
          }
        }}
        disabled={saving}
      />
      <div className="inline-category-row">
        <select
          aria-label="Tipo"
          value={kind}
          onChange={(e) => setKind(e.target.value as Category["kind"])}
          disabled={saving}
        >
          <option value="despesa">Despesa</option>
          <option value="receita">Receita</option>
        </select>
        <select
          aria-label="Natureza"
          value={nature}
          onChange={(e) => setNature(e.target.value as Category["nature"])}
          disabled={saving}
        >
          <option value="essencial">Essencial</option>
          <option value="variavel">Variável</option>
          <option value="investimento">Investimento</option>
        </select>
        <input
          type="color"
          className="color-input"
          aria-label="Cor"
          value={color}
          onChange={(e) => setColor(e.target.value)}
          disabled={saving}
        />
      </div>
      <div className="inline-category-actions">
        <button className="btn-text" type="button" onClick={onCancel} disabled={saving}>
          Cancelar
        </button>
        <button className="btn-outline" type="button" onClick={create} disabled={saving || !name.trim()}>
          {saving ? "Criando…" : "Criar"}
        </button>
      </div>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}

// A category select whose last option creates a new category in place; the
// new one is selected as soon as it exists.
export function CategorySelect({
  categories,
  value,
  onChange,
  onCreatingChange,
  id,
  name,
  className,
  required,
  disabled,
  emptyLabel = "Selecione",
  emptySelectable = false,
}: {
  categories: Category[];
  value: string;
  onChange: (id: string) => void;
  /** Lets the parent hold its submit while a category is being created. */
  onCreatingChange?: (creating: boolean) => void;
  id?: string;
  name?: string;
  className?: string;
  required?: boolean;
  disabled?: boolean;
  emptyLabel?: string;
  /** true when "no category" is a valid choice, not just a prompt. */
  emptySelectable?: boolean;
}) {
  // Categories created here show up before the page reloads its list.
  const [created, setCreated] = useState<Category[]>([]);
  const [creating, setCreating] = useState(false);
  const all = [...categories, ...created.filter((c) => !categories.some((k) => k.id === c.id))];

  function setCreatingBoth(v: boolean) {
    setCreating(v);
    onCreatingChange?.(v);
  }

  return (
    <>
      <select
        id={id}
        name={name}
        className={className}
        required={required}
        disabled={disabled}
        value={creating ? NEW_CATEGORY : value}
        onChange={(e) => {
          if (e.target.value === NEW_CATEGORY) {
            setCreatingBoth(true);
            return;
          }
          setCreatingBoth(false);
          onChange(e.target.value);
        }}
      >
        <option value="" disabled={!emptySelectable}>
          {emptyLabel}
        </option>
        {all.map((c) => (
          <option key={c.id} value={c.id}>
            {c.name}
          </option>
        ))}
        <option value={NEW_CATEGORY}>+ Nova categoria…</option>
      </select>
      {creating && (
        <InlineCategoryForm
          color={CATEGORY_COLORS[all.length % CATEGORY_COLORS.length]}
          onCancel={() => setCreatingBoth(false)}
          onCreated={(c) => {
            setCreated((prev) => [...prev, c]);
            setCreatingBoth(false);
            onChange(c.id);
          }}
        />
      )}
    </>
  );
}
