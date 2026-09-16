const MONTH_NAMES = [
  "janeiro", "fevereiro", "março", "abril", "maio", "junho",
  "julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
];

export function currentYearMonth(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

export function shiftYearMonth(yearMonth: string, delta: number): string {
  const [year, month] = yearMonth.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1 + delta, 1));
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}`;
}

export function formatYearMonth(yearMonth: string): string {
  const [year, month] = yearMonth.split("-").map(Number);
  const name = MONTH_NAMES[month - 1];
  return `${name.charAt(0).toUpperCase()}${name.slice(1)} de ${year}`;
}

export function formatYearMonthShort(yearMonth: string): string {
  const [year, month] = yearMonth.split("-").map(Number);
  return `${MONTH_NAMES[month - 1].slice(0, 3)}/${String(year).slice(2)}`;
}

// Credit-card billing math, ported by hand from
// backend/internal/domain/creditcard.go and backend/internal/domain/billing.go
// — those are the source of truth for the competence rule. If it ever
// changes there, mirror the change here too, or this hint will start lying
// again like it did before this file existed.
export interface CardBillingDays {
  closing_day: number;
  due_day: number;
}

// creditCardCompetenceYearMonth mirrors domain.CompetenceMonth for the
// credit-card branch: the purchase joins the invoice that closes on or
// after it (buying ON the closing day still makes that day's invoice), and
// that invoice's due date names the competence month.
export function creditCardCompetenceYearMonth(purchaseDate: string, card: CardBillingDays): string {
  const [year, month, day] = purchaseDate.split("-").map(Number);
  const closingMonthOffset = day > card.closing_day ? 1 : 0;
  const closing = new Date(Date.UTC(year, month - 1 + closingMonthOffset, card.closing_day));
  const dueMonthOffset = card.due_day <= card.closing_day ? 1 : 0;
  const due = new Date(Date.UTC(closing.getUTCFullYear(), closing.getUTCMonth() + dueMonthOffset, card.due_day));
  return `${due.getUTCFullYear()}-${String(due.getUTCMonth() + 1).padStart(2, "0")}`;
}

export interface CalendarDay {
  iso: string;
  dayNumber: number;
  inMonth: boolean;
  isToday: boolean;
}

// Sunday-start grid covering the full month plus the leading/trailing days
// needed to fill whole weeks, matching Google Calendar's month view.
export function buildMonthGrid(yearMonth: string): CalendarDay[] {
  const [year, month] = yearMonth.split("-").map(Number);
  const firstOfMonth = new Date(Date.UTC(year, month - 1, 1));
  const start = new Date(firstOfMonth);
  start.setUTCDate(start.getUTCDate() - start.getUTCDay());

  const lastOfMonth = new Date(Date.UTC(year, month, 0));
  const end = new Date(lastOfMonth);
  end.setUTCDate(end.getUTCDate() + (6 - end.getUTCDay()));

  const now = new Date();
  const todayIso = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;

  const days: CalendarDay[] = [];
  const cursor = new Date(start);
  while (cursor <= end) {
    const iso = `${cursor.getUTCFullYear()}-${String(cursor.getUTCMonth() + 1).padStart(2, "0")}-${String(cursor.getUTCDate()).padStart(2, "0")}`;
    days.push({
      iso,
      dayNumber: cursor.getUTCDate(),
      inMonth: cursor.getUTCMonth() === month - 1,
      isToday: iso === todayIso,
    });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return days;
}
