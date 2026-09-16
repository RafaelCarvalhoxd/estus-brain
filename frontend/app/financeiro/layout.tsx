import { ModuleTopBar } from "@/components/ModuleTopBar";
import { FinanceTabs } from "@/components/FinanceTabs";
import "../ui.css";

export default function FinanceiroLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <ModuleTopBar module="financeiro" />
      <main className="main">
        <div className="wrap">
          <FinanceTabs />
          {children}
        </div>
      </main>
    </div>
  );
}
