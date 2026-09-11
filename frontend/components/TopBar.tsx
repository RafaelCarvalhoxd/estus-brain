import Link from "next/link";
import { formatYearMonth, shiftYearMonth } from "@/lib/month";
import { IconChevronLeft, IconChevronRight, IconPlus } from "./icons";

export function TopBar({ month }: { month: string }) {
  return (
    <div className="topbar">
      <div className="month-nav">
        <Link href={`/?month=${shiftYearMonth(month, -1)}`} aria-label="Mês anterior">
          <IconChevronLeft />
        </Link>
        <h1>{formatYearMonth(month)}</h1>
        <Link href={`/?month=${shiftYearMonth(month, 1)}`} aria-label="Próximo mês">
          <IconChevronRight />
        </Link>
      </div>
      <a className="btn-primary" href="#novo-lancamento">
        <IconPlus />
        Novo lançamento
      </a>
    </div>
  );
}
