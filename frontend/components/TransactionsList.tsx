import type { Transaction } from "@/lib/types";
import { formatYearMonthShort } from "@/lib/month";

const PAYMENT_LABEL: Record<Transaction["payment_method"], string> = {
  debito: "débito",
  credito: "crédito",
  pix: "pix",
};

function meta(t: Transaction): string {
  const parts = [t.category_name, PAYMENT_LABEL[t.payment_method]];
  if (t.payment_method === "credito") {
    parts.push(`comprado em ${formatDay(t.purchase_date)}`);
  } else {
    parts.push(formatDay(t.purchase_date));
  }
  if (t.is_recurring) parts.push("recorrente");
  return parts.join(" · ");
}

function formatDay(iso: string): string {
  const [, month, day] = iso.split("-");
  return `${day}/${month}`;
}

export function TransactionsList({ transactions, month }: { transactions: Transaction[]; month: string }) {
  return (
    <div className="panel">
      <div className="panel-head">
        <h2>Lançamentos de {formatYearMonthShort(month)}</h2>
        <span>{transactions.length} itens</span>
      </div>
      {transactions.length === 0 ? (
        <p className="empty-note">Nenhum lançamento neste mês ainda. Use o formulário ao lado para começar.</p>
      ) : (
        transactions.map((t) => (
          <div className="txn" key={t.id}>
            <span className="txn-dot" style={{ background: t.category_color }} />
            <div className="txn-main">
              <div className="txn-title">
                {t.description}
                {t.installment_total && t.installment_total > 1
                  ? ` — parcela ${t.installment_number}/${t.installment_total}`
                  : ""}
              </div>
              <div className="txn-meta">{meta(t)}</div>
            </div>
            <div className="txn-amt">
              <div className="txn-value tab">{t.amount.formatted}</div>
              {t.payment_method === "credito" && (
                <span className="badge">fatura {formatYearMonthShort(month)}</span>
              )}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
