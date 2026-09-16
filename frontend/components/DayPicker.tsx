"use client";

import { DAY_LONG, DAY_SHORT, WEEK_ORDER } from "@/lib/week";

export function DayPicker({ value, onChange }: { value: number[]; onChange: (days: number[]) => void }) {
  const all = value.length === 7;
  return (
    <div className="day-picker" role="group" aria-label="Dias da semana">
      {WEEK_ORDER.map((d) => {
        const on = value.includes(d);
        return (
          <button
            key={d}
            type="button"
            className={`day-toggle${on ? " on" : ""}`}
            aria-pressed={on}
            aria-label={DAY_LONG[d]}
            onClick={() => onChange(on ? value.filter((x) => x !== d) : [...value, d].sort((a, b) => a - b))}
          >
            {DAY_SHORT[d]}
          </button>
        );
      })}
      <button type="button" className="btn-text" onClick={() => onChange(all ? [] : [0, 1, 2, 3, 4, 5, 6])}>
        {all ? "Limpar" : "Todos"}
      </button>
    </div>
  );
}
