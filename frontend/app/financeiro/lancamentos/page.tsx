import { getMonthSummary, listCategories, listCreditCards } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { TransactionsList } from "@/components/TransactionsList";
import { NewTransactionModal } from "@/components/NewTransactionModal";

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
        action={<NewTransactionModal categories={categories} cards={cards} />}
      />

      <TransactionsList transactions={summary.transactions} categories={categories} cards={cards} month={month} />
    </>
  );
}
