import type { WeekBucket } from "@/lib/types";

export function WeeklyChart({ weeks }: { weeks: WeekBucket[] }) {
  const max = Math.max(...weeks.map((w) => w.total.cents), 1);
  const nonZero = weeks.filter((w) => w.total.cents > 0).length;
  const average = nonZero > 0 ? weeks.reduce((sum, w) => sum + w.total.cents, 0) / nonZero : 0;
  const averageHeightPct = average > 0 ? (average / max) * 100 : 0;

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Ritmo semanal</h2>
        <span>por data efetiva</span>
      </div>
      {nonZero === 0 ? (
        <p className="empty-note">Sem lançamentos para distribuir por semana ainda.</p>
      ) : (
        <>
          <div className="week-chart">
            {weeks.map((w) => {
              const heightPct = Math.max((w.total.cents / max) * 100, w.total.cents > 0 ? 4 : 0);
              const aboveAverage = w.total.cents > average;
              return (
                <div className="week-col" key={w.label}>
                  <span className="week-val tab">
                    {(w.total.cents / 100).toLocaleString("pt-BR", { maximumFractionDigits: 0 })}
                  </span>
                  <div className="week-bar-wrap">
                    {average > 0 && <span className="avg-line" style={{ bottom: `${averageHeightPct}%` }} />}
                    <div className={`week-bar ${aboveAverage ? "avg-over" : ""}`} style={{ height: `${heightPct}%` }} />
                  </div>
                  <span className="week-lbl">{w.label}</span>
                </div>
              );
            })}
          </div>
          <p className="avg-caption">
            média semanal {(average / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })} · linha
            tracejada = média
          </p>
        </>
      )}
    </div>
  );
}
