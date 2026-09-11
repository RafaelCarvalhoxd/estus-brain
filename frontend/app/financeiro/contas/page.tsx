import { listBills, getBillSummary } from "@/lib/bills";
import type { Bill } from "@/lib/bills";
import { listCategories } from "@/lib/api";
import { BillForm } from "@/components/BillForm";
import { markBillPaidAction } from "./actions";
import "./bills.css";

function statusPillClass(status: Bill["status"]): string {
  if (status === "atrasado") return "pill bad";
  if (status === "pago" || status === "recebido") return "pill good";
  return "pill";
}

function statusLabel(status: Bill["status"]): string {
  switch (status) {
    case "atrasado":
      return "Atrasado";
    case "pago":
      return "Pago";
    case "recebido":
      return "Recebido";
    default:
      return "Pendente";
  }
}

function formatDueDate(dueDate: string): string {
  const [year, month, day] = dueDate.split("-");
  return `${day}/${month}/${year}`;
}

function BillList({ bills, direction }: { bills: Bill[]; direction: "pagar" | "receber" }) {
  if (bills.length === 0) {
    return <p className="empty-note">Nenhuma conta {direction === "pagar" ? "a pagar" : "a receber"}.</p>;
  }
  return (
    <div>
      {bills.map((bill) => (
        <div className="bill-row" key={bill.id}>
          <div className="bill-main">
            <div className="bill-title">{bill.description}</div>
            <div className="bill-meta">
              Vence em {formatDueDate(bill.due_date)}
              {bill.recurring ? " · recorrente" : ""}
            </div>
          </div>
          <span className={statusPillClass(bill.status)}>{statusLabel(bill.status)}</span>
          <span className="bill-amt tab">{bill.amount.formatted}</span>
          {!bill.paid_at && (
            <form action={markBillPaidAction.bind(null, bill.id)}>
              <button className="btn-outline" type="submit">
                {direction === "pagar" ? "Marcar como pago" : "Marcar como recebido"}
              </button>
            </form>
          )}
        </div>
      ))}
    </div>
  );
}

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
      </div>

      <section className="hero-row">
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

      <section className="bills-columns">
        <div className="panel">
          <div className="panel-head">
            <h2>A pagar</h2>
            <span>{payable.length} no total</span>
          </div>
          <BillList bills={payable} direction="pagar" />
        </div>
        <div className="panel">
          <div className="panel-head">
            <h2>A receber</h2>
            <span>{receivable.length} no total</span>
          </div>
          <BillList bills={receivable} direction="receber" />
        </div>
      </section>

      <BillForm categories={categories} />
    </>
  );
}
