import { getMonthSummary } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { HeroTiles } from "@/components/HeroTiles";
import { WeeklyChart } from "@/components/WeeklyChart";
import { CategoryBreakdown } from "@/components/CategoryBreakdown";

export default async function FinanceDashboardPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();
  const summary = await getMonthSummary(month);

  return (
    <>
      <TopBar month={month} basePath="/financeiro" />

      <HeroTiles summary={summary} />

      <section className="split">
        <CategoryBreakdown categories={summary.categories} />
        <WeeklyChart weeks={summary.weeks} />
      </section>
    </>
  );
}
