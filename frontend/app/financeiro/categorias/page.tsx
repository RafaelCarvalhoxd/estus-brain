import { getMonthSummary, listCategories } from "@/lib/api";
import { currentYearMonth } from "@/lib/month";
import { TopBar } from "@/components/TopBar";
import { CategoryBreakdown } from "@/components/CategoryBreakdown";
import { ComparisonTable } from "@/components/ComparisonTable";
import { CategoryManager } from "@/components/CategoryManager";

export default async function CategoriasPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string }>;
}) {
  const params = await searchParams;
  const month = params.month ?? currentYearMonth();
  const [summary, categories] = await Promise.all([getMonthSummary(month), listCategories()]);

  return (
    <>
      <TopBar month={month} basePath="/financeiro/categorias" />

      <div className="section-gap">
        <CategoryBreakdown categories={summary.categories} />
      </div>

      <div className="section-gap">
        <ComparisonTable comparison={summary.comparison} month={month} />
      </div>

      <CategoryManager categories={categories} />
    </>
  );
}
