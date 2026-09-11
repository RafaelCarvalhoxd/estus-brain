import type { CategoryComparison } from "@/lib/types";
import { formatYearMonthShort, shiftYearMonth } from "@/lib/month";
import { DeltaPill } from "./DeltaPill";

export function ComparisonTable({ comparison, month }: { comparison: CategoryComparison[]; month: string }) {
  const totalCurrent = comparison.reduce((sum, c) => sum + c.current.cents, 0);
  const totalPrevious = comparison.reduce((sum, c) => sum + c.previous.cents, 0);
  const anySpend = comparison.some((c) => c.current.cents > 0 || c.previous.cents > 0);

  return (
    <section className="panel table-panel">
      <div className="panel-head">
        <h2>Categoria a categoria</h2>
        <span>
          {formatYearMonthShort(month)} vs. {formatYearMonthShort(shiftYearMonth(month, -1))}
        </span>
      </div>
      {!anySpend ? (
        <p className="empty-note">Ainda não há dois meses de histórico para comparar.</p>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr>
                <th>Categoria</th>
                <th>{formatYearMonthShort(month)}</th>
                <th>{formatYearMonthShort(shiftYearMonth(month, -1))}</th>
                <th>Variação</th>
              </tr>
            </thead>
            <tbody>
              {comparison.map((c) => (
                <tr key={c.category_id}>
                  <td className="name-cell">
                    <span className="dot" style={{ background: c.color }} />
                    {c.name}
                  </td>
                  <td className="tab">{c.current.formatted}</td>
                  <td className="tab">{c.previous.formatted}</td>
                  <td>
                    <DeltaPill currentCents={c.current.cents} previousCents={c.previous.cents} />
                  </td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <td>Total</td>
                <td className="tab">
                  {(totalCurrent / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}
                </td>
                <td className="tab">
                  {(totalPrevious / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}
                </td>
                <td>
                  <DeltaPill currentCents={totalCurrent} previousCents={totalPrevious} />
                </td>
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </section>
  );
}
