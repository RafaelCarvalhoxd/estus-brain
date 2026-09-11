import type { MonthSummary } from "@/lib/types";
import { formatYearMonth, shiftYearMonth } from "@/lib/month";
import { DeltaPill } from "./DeltaPill";

export function HeroTiles({ summary }: { summary: MonthSummary }) {
  return (
    <section className="hero-row">
      <div className="tile">
        <p className="tile-label">Gasto do mês</p>
        <p className="tile-figure tab">{summary.total.formatted}</p>
        <p className="tile-sub">
          <DeltaPill currentCents={summary.total.cents} previousCents={summary.previous_month.cents} />
          vs. {summary.previous_month.formatted} em {formatYearMonth(shiftYearMonth(summary.month, -1))}
        </p>
      </div>
      <div className="tile">
        <p className="tile-label">Recorrentes</p>
        <p className="tile-figure tab">{summary.recurring.formatted}</p>
        <p className="tile-sub">gastos fixos do mês</p>
      </div>
      <div className="tile">
        <p className="tile-label">Parcelas em aberto</p>
        <p className="tile-figure tab">{summary.open_installments.formatted}</p>
        <p className="tile-sub">restante em meses futuros</p>
      </div>
      <div className="tile">
        <p className="tile-label">Próxima fatura</p>
        <p className="tile-figure tab">{summary.open_invoice.formatted}</p>
        <p className="tile-sub">acumulado no cartão até agora</p>
      </div>
    </section>
  );
}
