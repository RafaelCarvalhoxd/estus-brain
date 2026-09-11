import { getMonthSummary, listCategories, listCreditCards } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { HeroTiles } from "@/components/HeroTiles";
import { CategoryBreakdown } from "@/components/CategoryBreakdown";
import { WeeklyChart } from "@/components/WeeklyChart";
import { ComparisonTable } from "@/components/ComparisonTable";
import { TransactionsList } from "@/components/TransactionsList";
import { NewTransactionForm } from "@/components/NewTransactionForm";
import "./dashboard.css";

export default async function OverviewPage({
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
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <TopBar month={month} />

          <HeroTiles summary={summary} />

          <section className="split">
            <CategoryBreakdown categories={summary.categories} />
            <WeeklyChart weeks={summary.weeks} />
          </section>

          <ComparisonTable comparison={summary.comparison} month={month} />

          <section className="bottom-split">
            <TransactionsList transactions={summary.transactions} month={month} />
            <NewTransactionForm categories={categories} cards={cards} />
          </section>
        </div>
      </main>
    </div>
  );
}
