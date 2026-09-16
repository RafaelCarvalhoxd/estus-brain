// Days of the week, shared by Treino and Dieta. Numbering follows
// Date.getDay() and the API: 0 = Sunday … 6 = Saturday. Screens list the
// week Monday first, the way it's lived.

// The server may run in UTC; "today" is about where the owner lives.
export const TZ = "America/Sao_Paulo";

export const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0];
export const DAY_SHORT = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];
export const DAY_LONG = ["domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"];

const EN_SHORT = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export function weekdayIn(tz: string, date = new Date()): number {
  return EN_SHORT.indexOf(new Intl.DateTimeFormat("en-US", { weekday: "short", timeZone: tz }).format(date));
}

/** Minutes since midnight on the wall clock of `tz`. */
export function minutesIn(tz: string, date = new Date()): number {
  const parts = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone: tz }).formatToParts(date);
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? 0);
  return get("hour") * 60 + get("minute");
}

/** The calendar day in `tz` as YYYY-MM-DD — the form the API takes days in. */
export function dayKeyIn(tz: string, date = new Date()): string {
  return date.toLocaleDateString("en-CA", { timeZone: tz });
}

/** Moves a YYYY-MM-DD day by whole days, without any time zone involved. */
export function shiftDay(day: string, delta: number): string {
  const d = new Date(`${day}T12:00:00Z`);
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}

/** Weekday (0 = Sunday) of a YYYY-MM-DD day. */
export function weekdayOf(day: string): number {
  return new Date(`${day}T12:00:00Z`).getUTCDay();
}

export function timeToMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(":").map(Number);
  return (h || 0) * 60 + (m || 0);
}

export function daysLabel(days: number[]): string {
  const set = new Set(days);
  if (set.size === 7) return "Todos os dias";
  if (set.size === 5 && [1, 2, 3, 4, 5].every((d) => set.has(d))) return "Seg a Sex";
  if (set.size === 2 && set.has(0) && set.has(6)) return "Fim de semana";
  return WEEK_ORDER.filter((d) => set.has(d))
    .map((d) => DAY_SHORT[d])
    .join(", ");
}

export function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

// The Go API answers errors as `{"error": "..."}`; the fetch helpers wrap
// that in "estus-vault api <path> -> <status>: <body>". Pull the message
// back out so a form can show it.
export function apiErrorMessage(err: unknown, fallback: string): string {
  if (!(err instanceof Error)) return fallback;
  const match = /-> \d{3}: ([\s\S]*)$/.exec(err.message);
  if (match) {
    try {
      const body = JSON.parse(match[1]) as { error?: unknown };
      if (typeof body.error === "string") return `${fallback} (${body.error})`;
    } catch {
      // not JSON — fall through
    }
  }
  return fallback;
}
