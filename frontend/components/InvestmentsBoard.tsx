"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Investment, InvestmentKind, InvestmentList, RatePeriod } from "@/lib/investments";
import {
  contributeAction,
  createInvestmentAction,
  deleteContributionAction,
  deleteInvestmentAction,
  updateInvestmentAction,
  type InvestmentFields,
} from "@/app/financeiro/investimentos/actions";
import { Modal } from "./Modal";
import { IconPencil, IconTrash } from "./icons";

const KINDS: { id: InvestmentKind; label: string }[] = [
  { id: "cdb", label: "CDB" },
  { id: "tesouro", label: "Tesouro Direto" },
  { id: "lci_lca", label: "LCI / LCA" },
  { id: "poupanca", label: "Poupança" },
  { id: "fundo", label: "Fundo" },
  { id: "acoes", label: "Ações" },
  { id: "fii", label: "FII" },
  { id: "cripto", label: "Cripto" },
  { id: "outro", label: "Outro" },
];

const PERIODS: { id: RatePeriod; label: string }[] = [
  { id: "mes", label: "ao mês" },
  { id: "ano", label: "ao ano" },
];

function kindLabel(kind: InvestmentKind): string {
  return KINDS.find((k) => k.id === kind)?.label ?? kind;
}

function rateText(bp: number): string {
  return (bp / 100).toLocaleString("pt-BR", { minimumFractionDigits: 0, maximumFractionDigits: 2 });
}

function brl(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function formatDay(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function today(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function InvestmentFieldsForm({
  fields,
  onChange,
  disabled,
}: {
  fields: InvestmentFields;
  onChange: (f: InvestmentFields) => void;
  disabled: boolean;
}) {
  return (
    <>
      <div className="field">
        <label htmlFor="inv-name">Nome</label>
        <input
          id="inv-name"
          type="text"
          placeholder="Ex: CDB Nubank"
          value={fields.name}
          onChange={(e) => onChange({ ...fields, name: e.target.value })}
          disabled={disabled}
        />
      </div>
      <div className="field">
        <label htmlFor="inv-kind">Tipo</label>
        <select
          id="inv-kind"
          value={fields.kind}
          onChange={(e) => onChange({ ...fields, kind: e.target.value as InvestmentKind })}
          disabled={disabled}
        >
          {KINDS.map((k) => (
            <option key={k.id} value={k.id}>
              {k.label}
            </option>
          ))}
        </select>
      </div>
      <div className="row2">
        <div className="field">
          <label htmlFor="inv-rate">Rendimento (%)</label>
          <input
            id="inv-rate"
            type="text"
            inputMode="decimal"
            placeholder="1"
            value={fields.rate}
            onChange={(e) => onChange({ ...fields, rate: e.target.value })}
            disabled={disabled}
          />
        </div>
        <div className="field">
          <label>Período</label>
          <div className="seg">
            {PERIODS.map((p) => (
              <button
                key={p.id}
                type="button"
                className={fields.ratePeriod === p.id ? "active" : ""}
                onClick={() => onChange({ ...fields, ratePeriod: p.id })}
                disabled={disabled}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}

function AmountDateFields({
  amount,
  date,
  onAmount,
  onDate,
  disabled,
}: {
  amount: string;
  date: string;
  onAmount: (v: string) => void;
  onDate: (v: string) => void;
  disabled: boolean;
}) {
  return (
    <div className="row2">
      <div className="field">
        <label htmlFor="inv-amount">Valor</label>
        <input
          id="inv-amount"
          type="text"
          inputMode="decimal"
          placeholder="0,00"
          value={amount}
          onChange={(e) => onAmount(e.target.value)}
          disabled={disabled}
        />
      </div>
      <div className="field">
        <label htmlFor="inv-date">Data do aporte</label>
        <input id="inv-date" type="date" value={date} onChange={(e) => onDate(e.target.value)} disabled={disabled} />
      </div>
    </div>
  );
}

function FormFooter({
  error,
  saving,
  canSave,
  onSave,
  onCancel,
  label,
}: {
  error: string | null;
  saving: boolean;
  canSave: boolean;
  onSave: () => void;
  onCancel: () => void;
  label: string;
}) {
  return (
    <>
      {error && <p className="form-error">{error}</p>}
      <div className="row-actions" style={{ justifyContent: "flex-end" }}>
        <button className="btn-text" type="button" onClick={onCancel} disabled={saving}>
          Cancelar
        </button>
        <button className="btn-block" type="button" onClick={onSave} disabled={saving || !canSave}>
          {saving ? "Salvando…" : label}
        </button>
      </div>
    </>
  );
}

function useSave(onDone: () => void) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function run(action: () => Promise<{ error?: string }>) {
    setSaving(true);
    setError(null);
    const result = await action();
    setSaving(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    router.refresh();
    onDone();
  }
  return { saving, error, run };
}

function NewInvestmentForm({ onClose }: { onClose: () => void }) {
  const [fields, setFields] = useState<InvestmentFields>({ name: "", kind: "cdb", rate: "", ratePeriod: "mes" });
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(today);
  const { saving, error, run } = useSave(onClose);

  return (
    <div className="form-grid">
      <InvestmentFieldsForm fields={fields} onChange={setFields} disabled={saving} />
      <AmountDateFields amount={amount} date={date} onAmount={setAmount} onDate={setDate} disabled={saving} />
      <p className="empty-note">O valor sai do saldo do mês do aporte.</p>
      <FormFooter
        error={error}
        saving={saving}
        canSave={!!fields.name.trim() && !!amount.trim()}
        onSave={() => run(() => createInvestmentAction(fields, amount, date))}
        onCancel={onClose}
        label="Investir"
      />
    </div>
  );
}

function EditInvestmentForm({ investment, onClose }: { investment: Investment; onClose: () => void }) {
  const [fields, setFields] = useState<InvestmentFields>({
    name: investment.name,
    kind: investment.kind,
    rate: investment.rate_bp ? rateText(investment.rate_bp) : "",
    ratePeriod: investment.rate_period,
  });
  const { saving, error, run } = useSave(onClose);

  return (
    <div className="form-grid">
      <InvestmentFieldsForm fields={fields} onChange={setFields} disabled={saving} />
      <FormFooter
        error={error}
        saving={saving}
        canSave={!!fields.name.trim()}
        onSave={() => run(() => updateInvestmentAction(investment.id, fields))}
        onCancel={onClose}
        label="Salvar"
      />
    </div>
  );
}

function ContributeForm({ investment, onClose }: { investment: Investment; onClose: () => void }) {
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(today);
  const { saving, error, run } = useSave(onClose);

  return (
    <div className="form-grid">
      <AmountDateFields amount={amount} date={date} onAmount={setAmount} onDate={setDate} disabled={saving} />
      <p className="empty-note">O valor sai do saldo do mês do aporte.</p>
      <FormFooter
        error={error}
        saving={saving}
        canSave={!!amount.trim()}
        onSave={() => run(() => contributeAction(investment.id, amount, date))}
        onCancel={onClose}
        label="Aportar"
      />
    </div>
  );
}

type Dialog = { mode: "new" } | { mode: "edit" | "contribute"; investment: Investment } | null;

function InvestmentRow({ investment, onDialog }: { investment: Investment; onDialog: (d: Dialog) => void }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const earned = investment.estimated.cents - investment.invested.cents;

  async function remove(action: () => Promise<{ error?: string }>) {
    setBusy(true);
    setError(null);
    const result = await action();
    setBusy(false);
    if (result.error) setError(result.error);
    else router.refresh();
  }

  function removeInvestment() {
    if (!confirm(`Excluir "${investment.name}"? Os aportes voltam para o saldo dos meses em que foram feitos.`)) return;
    remove(() => deleteInvestmentAction(investment.id));
  }

  return (
    <div className="inv-row">
      <div className="inv-main">
        <button
          type="button"
          className="inv-toggle"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          aria-label={`${open ? "Esconder" : "Mostrar"} aportes de ${investment.name}`}
        >
          <span className="inv-name">{investment.name}</span>
          <span className="inv-meta">
            {kindLabel(investment.kind)} · {rateText(investment.rate_bp)}%{" "}
            {investment.rate_period === "mes" ? "ao mês" : "ao ano"} · {investment.contributions.length}{" "}
            {investment.contributions.length === 1 ? "aporte" : "aportes"}
          </span>
        </button>
        <div className="inv-figures tab">
          <span className="inv-estimated">{investment.estimated.formatted}</span>
          <span className="inv-invested">
            aplicado {investment.invested.formatted}
            {earned > 0 && <span className="inv-earned"> +{brl(earned)}</span>}
          </span>
        </div>
        <div className="row-actions">
          <button
            className="btn-text"
            type="button"
            onClick={() => onDialog({ mode: "contribute", investment })}
            disabled={busy}
          >
            + Aporte
          </button>
          <button
            className="icon-btn"
            type="button"
            aria-label="Editar investimento"
            onClick={() => onDialog({ mode: "edit", investment })}
            disabled={busy}
          >
            <IconPencil />
          </button>
          <button className="icon-btn bad" type="button" aria-label="Excluir investimento" onClick={removeInvestment} disabled={busy}>
            <IconTrash />
          </button>
        </div>
      </div>
      {open && (
        <ul className="inv-contributions">
          {[...investment.contributions].reverse().map((c) => (
            <li key={c.id}>
              <span>{formatDay(c.date)}</span>
              <span className="tab">{c.amount.formatted}</span>
              <button
                className="icon-btn bad"
                type="button"
                aria-label={`Excluir aporte de ${formatDay(c.date)}`}
                disabled={busy}
                onClick={() => {
                  if (confirm(`Excluir o aporte de ${c.amount.formatted}? O valor volta para o saldo daquele mês.`)) {
                    remove(() => deleteContributionAction(investment.id, c.id));
                  }
                }}
              >
                <IconTrash />
              </button>
            </li>
          ))}
        </ul>
      )}
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}

export function InvestmentsBoard({ data }: { data: InvestmentList }) {
  const [dialog, setDialog] = useState<Dialog>(null);
  const close = () => setDialog(null);
  const earned = data.estimated.cents - data.invested.cents;

  return (
    <>
      <section className="kpi-grid">
        <div className="tile">
          <p className="tile-label">Aplicado</p>
          <p className="tile-figure tab">{data.invested.formatted}</p>
          <p className="tile-sub">soma dos aportes</p>
        </div>
        <div className="tile">
          <p className="tile-label">Valor estimado hoje</p>
          <p className="tile-figure tab">{data.estimated.formatted}</p>
          <p className="tile-sub">aportes com o rendimento de cada um</p>
        </div>
        <div className="tile">
          <p className="tile-label">Rendimento estimado</p>
          <p className="tile-figure tab">{brl(earned)}</p>
          <p className="tile-sub">valor estimado − aplicado</p>
        </div>
      </section>

      <div className="panel">
        <div className="panel-head">
          <h2>Investimentos</h2>
          <button className="btn-text" type="button" onClick={() => setDialog({ mode: "new" })}>
            + Novo investimento
          </button>
        </div>
        {data.investments.length === 0 ? (
          <p className="empty-note">Nenhum investimento ainda.</p>
        ) : (
          data.investments.map((inv) => <InvestmentRow key={inv.id} investment={inv} onDialog={setDialog} />)
        )}
      </div>

      <Modal open={dialog !== null} onClose={close}>
        <div className="panel">
          <div className="panel-head">
            <h2>
              {dialog?.mode === "new"
                ? "Novo investimento"
                : dialog?.mode === "edit"
                  ? "Editar investimento"
                  : `Aporte em ${dialog?.investment.name ?? ""}`}
            </h2>
          </div>
          {dialog?.mode === "new" && <NewInvestmentForm onClose={close} />}
          {dialog?.mode === "edit" && <EditInvestmentForm investment={dialog.investment} onClose={close} />}
          {dialog?.mode === "contribute" && <ContributeForm investment={dialog.investment} onClose={close} />}
        </div>
      </Modal>
    </>
  );
}
