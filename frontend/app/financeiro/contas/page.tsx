import { listBills, getBillSummary } from "@/lib/bills";
import { listCategories } from "@/lib/api";
import { BillsBoard } from "@/components/BillsBoard";
import { NewBillModal } from "@/components/NewBillModal";
import "./bills.css";

export default async function BillsPage() {
  const [payable, receivable, summary, categories] = await Promise.all([
    listBills("pagar"),
    listBills("receber"),
    getBillSummary(),
    listCategories(),
  ]);

  return (
    <>
      <div className="topbar">
        <h1 className="page-title">Contas a pagar e a receber</h1>
        <NewBillModal categories={categories} />
      </div>

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

      <BillsBoard payable={payable} receivable={receivable} categories={categories} />
    </>
  );
}
