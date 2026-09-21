import Link from "next/link";
import { getMonthSummary } from "@/lib/api";
import { getBillSummary, getBillsReceived } from "@/lib/bills";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { CategoryPieChart } from "@/components/CategoryPieChart";

function formatSigned(cents: number): string {
  const value = cents / 100;
  return value.toLocaleString("pt-BR", { style: "currency", currency: "BRL", signDisplay: "auto" });
}

export default async function FinanceDashboardPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();

  const [summary, billSummary, received] = await Promise.all([
    getMonthSummary(month),
    getBillSummary(),
    getBillsReceived(month),
  ]);

  const saidas = summary.total.cents;
  const entradas = received.cents;
  const saldo = entradas - saidas;

  return (
    <>
      <TopBar month={month} basePath="/financeiro" />

      <section className="kpi-grid">
        <div className="tile">
          <p className="tile-label">Entradas</p>
          <p className="tile-figure tab">{received.formatted}</p>
          <p className="tile-sub">recebido no mês</p>
        </div>
        <div className="tile">
          <p className="tile-label">Saídas</p>
          <p className="tile-figure tab">{summary.total.formatted}</p>
          <p className="tile-sub">gasto no mês</p>
        </div>
        <div className="tile">
          <p className="tile-label">Gastos fixos</p>
          <p className="tile-figure tab">{summary.recurring.formatted}</p>
          <p className="tile-sub">do total do mês</p>
        </div>
        <div className="tile">
          <p className="tile-label">Gastos variáveis</p>
          <p className="tile-figure tab">{summary.variable.formatted}</p>
          <p className="tile-sub">do total do mês</p>
        </div>
        <div className="tile">
          <p className="tile-label">Saldo</p>
          <p className="tile-figure tab">{formatSigned(saldo)}</p>
          <p className="tile-sub">
            <span className={`pill ${saldo >= 0 ? "good" : "bad"}`}>{saldo >= 0 ? "positivo" : "negativo"}</span>
          </p>
        </div>

        <div className="tile">
          <p className="tile-label">CP em aberto</p>
          <p className="tile-figure tab">{billSummary.payable_open.formatted}</p>
          <p className="tile-sub">
            <Link className="btn-text" href="/financeiro/contas">
              Ver contas
            </Link>
          </p>
        </div>
        <div className="tile">
          <p className="tile-label">CR em aberto</p>
          <p className="tile-figure tab">{billSummary.receivable_open.formatted}</p>
          <p className="tile-sub">
            <Link className="btn-text" href="/financeiro/contas">
              Ver contas
            </Link>
          </p>
        </div>
        <div className="tile">
          <p className="tile-label">Contas vencidas</p>
          <p className="tile-figure tab">{billSummary.overdue_count}</p>
          {billSummary.overdue_count > 0 && (
            <p className="tile-sub">
              <span className="pill bad">precisa de atenção</span>
            </p>
          )}
        </div>
      </section>

      <CategoryPieChart categories={summary.categories} />
    </>
  );
}
