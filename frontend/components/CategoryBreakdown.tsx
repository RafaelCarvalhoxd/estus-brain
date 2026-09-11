import type { CategorySlice } from "@/lib/types";

export function CategoryBreakdown({ categories }: { categories: CategorySlice[] }) {
  const withSpend = categories.filter((c) => c.total.cents > 0);
  const max = Math.max(...categories.map((c) => c.total.cents), 1);

  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Gasto por categoria</h2>
        <span>valor absoluto</span>
      </div>
      {withSpend.length === 0 ? (
        <p className="empty-note">Nenhum gasto lançado neste mês ainda.</p>
      ) : (
        withSpend.map((c) => (
          <div className="cat-row" key={c.category_id}>
            <span className="cat-name">
              <span className="dot" style={{ background: c.color }} />
              {c.name}
            </span>
            <div className="bar-track">
              <div
                className="bar-fill"
                style={{ width: `${Math.max((c.total.cents / max) * 100, 4)}%`, background: c.color }}
              />
            </div>
            <span className="cat-amt tab">{c.total.formatted}</span>
          </div>
        ))
      )}
    </div>
  );
}
