import type { Money } from "@/lib/types";

const brl = (cents: number) => (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

function percent(part: number, total: number): number {
  return total > 0 ? Math.round((part / total) * 100) : 0;
}

const R = 52;
const STROKE = 16;
const CIRC = 2 * Math.PI * R;
// Surface gap between the two arcs, in the same units as the circumference.
const GAP = 3;

function Donut({ fixed, variable }: { fixed: number; variable: number }) {
  const total = fixed + variable;
  const both = fixed > 0 && variable > 0;
  const arc = (value: number) => Math.max((value / total) * CIRC - (both ? GAP : 0), 0);
  const fixedLen = arc(fixed);
  const variableLen = arc(variable);
  return (
    <svg className="fv-donut" viewBox="0 0 140 140" aria-hidden="true">
      <circle cx="70" cy="70" r={R} className="fv-donut-track" strokeWidth={STROKE} />
      {fixed > 0 && (
        <circle
          cx="70"
          cy="70"
          r={R}
          className="fv-arc is-fixed"
          strokeWidth={STROKE}
          strokeDasharray={`${fixedLen} ${CIRC}`}
          transform="rotate(-90 70 70)"
        >
          <title>Fixos: {brl(fixed)}</title>
        </circle>
      )}
      {variable > 0 && (
        <circle
          cx="70"
          cy="70"
          r={R}
          className="fv-arc is-variable"
          strokeWidth={STROKE}
          strokeDasharray={`${variableLen} ${CIRC}`}
          strokeDashoffset={-(fixedLen + (both ? GAP : 0))}
          transform="rotate(-90 70 70)"
        >
          <title>Variáveis: {brl(variable)}</title>
        </circle>
      )}
    </svg>
  );
}

// Month's spending split into fixed (recurring expenses plus recurring bills
// still to pay) and variable.
export function FixedVariableSplit({
  recurring,
  openFixed,
  variable,
  received,
}: {
  recurring: Money;
  openFixed: Money;
  variable: Money;
  received: Money;
}) {
  const fixed = recurring.cents + openFixed.cents;
  const total = fixed + variable.cents;
  const fixedPct = percent(fixed, total);
  const variablePct = total > 0 ? 100 - fixedPct : 0;
  const paidShare = fixed > 0 ? (recurring.cents / fixed) * 100 : 0;
  const ofIncome = received.cents > 0 ? percent(fixed, received.cents) : null;

  return (
    <section className="panel fv-panel">
      <div className="panel-head">
        <h2>Fixos × variáveis</h2>
        <span>contas recorrentes contam como fixas</span>
      </div>
      {total === 0 ? (
        <p className="empty-note">Nenhum gasto neste mês ainda.</p>
      ) : (
        <div className="fv-body">
          <div
            className="fv-chart"
            role="img"
            aria-label={`Fixos ${fixedPct}%, ${brl(fixed)}. Variáveis ${variablePct}%, ${variable.formatted}.`}
          >
            <Donut fixed={fixed} variable={variable.cents} />
            <div className="fv-center">
              <b className="tab">{brl(total)}</b>
              <span>no mês</span>
            </div>
          </div>

          <div className="fv-legend">
            <div className="fv-item is-fixed">
              <div className="fv-item-head">
                <span className="fv-swatch is-fixed" />
                <span className="fv-name">Fixos</span>
                <span className="fv-amt tab">{brl(fixed)}</span>
              </div>
              <p className="fv-pct tab">{fixedPct}%</p>
              {fixed > 0 && (
                <>
                  <div className="fv-progress" aria-hidden="true">
                    <div className="fv-progress-paid" style={{ width: `${paidShare}%` }} />
                  </div>
                  <p className="fv-sub">
                    {recurring.formatted} pago · {openFixed.formatted} a pagar
                  </p>
                </>
              )}
            </div>
            <div className="fv-item is-variable">
              <div className="fv-item-head">
                <span className="fv-swatch is-variable" />
                <span className="fv-name">Variáveis</span>
                <span className="fv-amt tab">{variable.formatted}</span>
              </div>
              <p className="fv-pct tab">{variablePct}%</p>
              <p className="fv-sub">compras e gastos avulsos</p>
            </div>
          </div>

          {ofIncome !== null && (
            <p className="fv-insight">
              Os fixos comprometem <b className="tab">{ofIncome}%</b> do que entrou no mês.
            </p>
          )}
        </div>
      )}
    </section>
  );
}
