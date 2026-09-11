import { getMonthSummary, listCategories, listCreditCards } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { TransactionsList } from "@/components/TransactionsList";
import { NewTransactionForm } from "@/components/NewTransactionForm";
import { IconPlus } from "@/components/icons";

export default async function LancamentosPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();

  const [summary, categories, cards] = await Promise.all([
    getMonthSummary(month),
    listCategories(),
    listCreditCards(),
  ]);

  return (
    <>
      <TopBar
        month={month}
        basePath="/financeiro/lancamentos"
        action={
          <a className="btn-primary" href="#novo-lancamento">
            <IconPlus />
            Novo lançamento
          </a>
        }
      />

      <section className="bottom-split">
        <TransactionsList transactions={summary.transactions} month={month} />
        <NewTransactionForm categories={categories} cards={cards} />
      </section>
    </>
  );
}
