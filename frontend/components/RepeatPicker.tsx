"use client";

import type { Reminder } from "@/lib/reminders";
import type { Repeat } from "@/app/lembretes/actions";

export const WEEKDAYS = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

export function formatRepeat(r: Reminder): string | null {
  if (r.repeat_month_day) return `Todo dia ${r.repeat_month_day}`;
  if (r.repeat_days.length === 0) return null;
  if (r.repeat_days.length === 7) return "Todo dia";
  return r.repeat_days.map((d) => WEEKDAYS[d]).join(", ");
}

type RepeatMode = "none" | "weekly" | "monthly";
export type RepeatValue = { mode: RepeatMode; days: number[]; monthDay: number };

export function repeatValueOf(r?: Reminder): RepeatValue {
  const today = new Date().getDate();
  if (r?.repeat_month_day) return { mode: "monthly", days: [], monthDay: r.repeat_month_day };
  if (r && r.repeat_days.length > 0) return { mode: "weekly", days: r.repeat_days, monthDay: today };
  return { mode: "none", days: [], monthDay: today };
}

export function toRepeat(v: RepeatValue): Repeat {
  if (v.mode === "monthly") return { days: [], monthDay: v.monthDay };
  if (v.mode === "weekly") return { days: v.days };
  return { days: [] };
}

// How a reminder repeats. With `named`, the choice also goes into the form
// as hidden inputs.
export function RepeatPicker({
  value,
  onChange,
  named,
  disabled,
}: {
  value: RepeatValue;
  onChange: (v: RepeatValue) => void;
  named?: boolean;
  disabled?: boolean;
}) {
  const repeat = toRepeat(value);
  function toggle(day: number) {
    const days = value.days.includes(day) ? value.days.filter((d) => d !== day) : [...value.days, day].sort();
    onChange({ ...value, days });
  }
  return (
    <div className="repeat-picker">
      <select
        className="txn-edit-input"
        aria-label="Repetir"
        value={value.mode}
        onChange={(e) => onChange({ ...value, mode: e.target.value as RepeatMode })}
        disabled={disabled}
      >
        <option value="none">Não repete</option>
        <option value="weekly">Dias da semana</option>
        <option value="monthly">Todo mês</option>
      </select>
      {value.mode === "weekly" && (
        <div className="weekday-picker" role="group" aria-label="Dias da semana">
          {WEEKDAYS.map((label, day) => (
            <button
              key={day}
              type="button"
              className={`weekday-chip${value.days.includes(day) ? " is-on" : ""}`}
              aria-pressed={value.days.includes(day)}
              onClick={() => toggle(day)}
              disabled={disabled}
            >
              {label}
            </button>
          ))}
        </div>
      )}
      {value.mode === "monthly" && (
        <label className="repeat-month-day">
          no dia
          <input
            className="txn-edit-input"
            type="number"
            min={1}
            max={31}
            value={value.monthDay}
            onChange={(e) => onChange({ ...value, monthDay: Math.min(31, Math.max(1, Number(e.target.value) || 1)) })}
            disabled={disabled}
          />
        </label>
      )}
      {named && repeat.days.map((d) => <input key={d} type="hidden" name="repeat_days" value={d} />)}
      {named && repeat.monthDay && <input type="hidden" name="repeat_month_day" value={repeat.monthDay} />}
    </div>
  );
}
