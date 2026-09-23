import Link from "next/link";
import { getMonthSummary } from "@/lib/api";
import { getBillSummary, getBillsReceived } from "@/lib/bills";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { CategoryPieChart } from "@/components/CategoryPieChart";
import { FixedVariableSplit } from "@/components/FixedVariableSplit";

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

  // The month summary materializes the month's recurring bills, so the bill
  // totals are read only after it.
  const [summary, received] = await Promise.all([getMonthSummary(month), getBillsReceived(month)]);
  const billSummary = await getBillSummary(month);

  // Only money that actually moved: card purchases wait for their invoice
  // to be paid, and open bills don't count until settled.
  const saldo = received.cents - summary.paid_out.cents;

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
          <p className="tile-figure tab">{summary.paid_out.formatted}</p>
          <p className="tile-sub">débito, pix e faturas pagas</p>
        </div>
        <div className="tile">
          <p className="tile-label">Saldo</p>
          <p className="tile-figure tab">{formatSigned(saldo)}</p>
          <p className="tile-sub">
            <span className={`pill ${saldo >= 0 ? "good" : "bad"}`}>{saldo >= 0 ? "positivo" : "negativo"}</span>
            entradas − saídas
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

      <FixedVariableSplit
        recurring={summary.recurring}
        openFixed={summary.open_fixed}
        variable={summary.variable}
        received={received}
      />

      <CategoryPieChart categories={summary.categories} />
    </>
  );
}
