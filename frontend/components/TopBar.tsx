import type { ReactNode } from "react";
import Link from "next/link";
import { formatYearMonth, shiftYearMonth } from "@/lib/month";
import { IconChevronLeft, IconChevronRight } from "./icons";

export function TopBar({
  month,
  basePath,
  action,
}: {
  month: string;
  basePath: string;
  action?: ReactNode;
}) {
  return (
    <div className="topbar">
      <div className="month-nav">
        <Link href={`${basePath}?month=${shiftYearMonth(month, -1)}`} aria-label="Mês anterior">
          <IconChevronLeft />
        </Link>
        <h1 className="page-title">{formatYearMonth(month)}</h1>
        <Link href={`${basePath}?month=${shiftYearMonth(month, 1)}`} aria-label="Próximo mês">
          <IconChevronRight />
        </Link>
      </div>
      {action}
    </div>
  );
}
