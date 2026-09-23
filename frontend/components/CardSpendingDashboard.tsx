"use client";

import { useState } from "react";
import type { CardInvoice, CardOverview } from "@/lib/types";

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

function monthLabel(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString("pt-BR", { month: "short" }).replace(".", "");
}

function dayMonth(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString("pt-BR", { day: "2-digit", month: "short" }).replace(".", "");
}

function InvoiceBars({ months, current }: { months: CardInvoice[]; current: string }) {
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(...months.map((m) => m.total.cents), 1);
  return (
    <div className="card-bars" role="list" aria-label="Fatura por mês" onMouseLeave={() => setHover(null)}>
      {months.map((m, i) => {
        const kind = m.month === current ? "is-current" : m.month > current ? "is-future" : "is-past";
        return (
          <div
            key={m.month}
            className={`card-bar ${kind}${hover === i ? " is-hover" : ""}`}
            role="listitem"
            aria-label={`${monthLabel(m.month)}: ${m.total.formatted}`}
            onMouseEnter={() => setHover(i)}
            onFocus={() => setHover(i)}
            tabIndex={0}
          >
            {hover === i && (
              <div className="card-bar-tip">
                <b className="tab">{m.total.formatted}</b>
                <span>
                  {m.purchases} {m.purchases === 1 ? "compra" : "compras"} · vence {dayMonth(m.due_date)}
                  {m.paid ? " · paga" : ""}
                </span>
              </div>
            )}
            <div className="card-bar-track">
              <div className="card-bar-fill" style={{ height: `${m.total.cents > 0 ? Math.max((m.total.cents / max) * 100, 3) : 0}%` }} />
            </div>
            <span className="card-bar-label">{monthLabel(m.month)}</span>
          </div>
        );
      })}
    </div>
  );
}

function CardPanel({ ov }: { ov: CardOverview }) {
  const { card, current, categories } = ov;
  const max = Math.max(...categories.map((c) => c.total.cents), 1);
  return (
    <section className="panel card-panel">
      <div className="card-panel-head">
        <div>
          <h2>{card.name}</h2>
          <p className="card-panel-meta">
            Fecha dia {card.closing_day} · vence dia {card.due_day}
          </p>
        </div>
        <span className={`pill${current.paid ? " good" : ""}`}>{current.paid ? "Paga" : "Em aberto"}</span>
      </div>

      <div className="card-hero">
        <p className="tile-label">Fatura de {monthLabel(current.month)}</p>
        <p className="card-hero-figure tab">{current.total.formatted}</p>
        <p className="tile-sub">
          Vence {dayMonth(current.due_date)} · {current.purchases} {current.purchases === 1 ? "compra" : "compras"}
        </p>
      </div>

      <InvoiceBars months={ov.months} current={current.month} />

      <div className="card-cats">
        <h3>Por categoria nesta fatura</h3>
        {categories.length === 0 ? (
          <p className="empty-note">Nenhuma compra nesta fatura ainda.</p>
        ) : (
          categories.map((c) => (
            <div className="cat-row" key={c.name}>
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
    </section>
  );
}

export function CardSpendingDashboard({ overviews }: { overviews: CardOverview[] }) {
  if (overviews.length === 0) return null;
  const open = overviews.filter((o) => !o.current.paid);
  const openTotal = open.reduce((sum, o) => sum + o.current.total.cents, 0);
  const next = [...open].filter((o) => o.current.total.cents > 0).sort((a, b) => a.current.due_date.localeCompare(b.current.due_date))[0];
  const biggest = [...overviews].sort((a, b) => b.current.total.cents - a.current.total.cents)[0];

  return (
    <div className="card-dash">
      <div className="kpi-grid">
        <div className="tile">
          <p className="tile-label">Faturas em aberto</p>
          <p className="tile-figure tab">{brl.format(openTotal / 100)}</p>
          <p className="tile-sub">
            {open.length} de {overviews.length} {overviews.length === 1 ? "cartão" : "cartões"}
          </p>
        </div>
        <div className="tile">
          <p className="tile-label">Próximo vencimento</p>
          <p className="tile-figure">{next ? dayMonth(next.current.due_date) : "—"}</p>
          <p className="tile-sub">{next ? `${next.card.name} · ${next.current.total.formatted}` : "Nada a pagar"}</p>
        </div>
        <div className="tile">
          <p className="tile-label">Maior fatura</p>
          <p className="tile-figure tab">{biggest.current.total.formatted}</p>
          <p className="tile-sub">{biggest.card.name}</p>
        </div>
      </div>
      <div className="card-dash-grid">
        {overviews.map((ov) => (
          <CardPanel key={ov.card.id} ov={ov} />
        ))}
      </div>
    </div>
  );
}
