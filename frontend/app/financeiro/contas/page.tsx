import { listBills, getBillSummary } from "@/lib/bills";
import { listCategories, listCreditCards } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { BillsBoard } from "@/components/BillsBoard";
import { NewBillModal } from "@/components/NewBillModal";
import "./bills.css";

export default async function BillsPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();
  // One call for the month, split by direction here instead of two separate
  // ?direction= requests: GET /api/bills?month= materializes ym's recurring
  // series as a side effect, and two of those requests firing in parallel
  // (Promise.all, one per direction) would race — both reading "nothing for
  // this month yet" before either write commits, each creating its own
  // occurrence. A single request removes that race entirely.
  const [bills, summary, categories, cards] = await Promise.all([
    listBills(undefined, month),
    getBillSummary(),
    listCategories(),
    listCreditCards(),
  ]);
  const payable = bills.filter((b) => b.direction === "pagar");
  const receivable = bills.filter((b) => b.direction === "receber");

  return (
    <>
      <TopBar
        month={month}
        basePath="/financeiro/contas"
        action={<NewBillModal categories={categories} cards={cards} />}
      />

      <section className="kpi-grid">
        <div className="tile">
          <p className="tile-label">A pagar em aberto</p>
          <p className="tile-figure tab">{summary.payable_open.formatted}</p>
        </div>
        <div className="tile">
          <p className="tile-label">A receber em aberto</p>
          <p className="tile-figure tab">{summary.receivable_open.formatted}</p>
        </div>
        <div className="tile">
          <p className="tile-label">Atrasados</p>
          <p className="tile-figure tab">{summary.overdue_count}</p>
          {summary.overdue_count > 0 && (
            <p className="tile-sub">
              <span className="pill bad">precisa de atenção</span>
            </p>
          )}
        </div>
      </section>

      <BillsBoard payable={payable} receivable={receivable} categories={categories} cards={cards} />
    </>
  );
}
