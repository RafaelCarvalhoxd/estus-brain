import { getMonthSummary } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { CategoryBreakdown } from "@/components/CategoryBreakdown";
import { ComparisonTable } from "@/components/ComparisonTable";

export default async function CategoriasPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();
  const summary = await getMonthSummary(month);

  return (
    <>
      <TopBar month={month} basePath="/financeiro/categorias" />

      <div className="section-gap">
        <CategoryBreakdown categories={summary.categories} />
      </div>

      <ComparisonTable comparison={summary.comparison} month={month} />
    </>
  );
}
