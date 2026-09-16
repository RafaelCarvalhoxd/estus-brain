"use client";

import { Fragment, useEffect, useMemo, useOptimistic, useRef, useState, useTransition, type CSSProperties } from "react";
import type { Habit, HabitInput, HabitKind } from "@/lib/habits";
import { deleteHabitAction, saveHabitAction, setHabitLogAction } from "@/app/habitos/actions";
import { DAY_SHORT, WEEK_ORDER, capitalize, daysLabel, shiftDay, weekdayOf } from "@/lib/week";
import { Modal } from "../Modal";
import { DayPicker } from "../DayPicker";
import { IconPencil, IconPlus } from "../icons";

type Logs = Record<string, Record<string, number>>;

const COLORS = ["#e2c23a", "#7cc242", "#10a37f", "#0f9aa8", "#3b7fe0", "#8b5cf6", "#d062c8", "#e8553d"];
const MONTHS = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"];

function dayLabel(day: string): string {
  const d = new Date(`${day}T12:00:00Z`);
  return capitalize(d.toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long", timeZone: "UTC" }));
}

function scheduled(h: Habit, day: string): boolean {
  return h.days.includes(weekdayOf(day)) && day >= h.start_day;
}

function IconFlame() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 21c-3.9 0-6.5-2.6-6.5-6.2 0-3.2 2.3-5.3 3.6-7.6.3 1.6 1 2.7 2.1 3.3.3-3.2 1.7-5.6 3.6-7.5.4 3 1.6 4.9 2.8 6.6 1 1.4 1.9 3 1.9 5.2 0 3.6-3.1 6.2-7.5 6.2Z" />
    </svg>
  );
}

function IconCheck() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="m5 12.5 4.5 4.5L19 7.5" />
    </svg>
  );
}

function Streak({ habit }: { habit: Habit }) {
  const n = habit.current_streak;
  return (
    <span
      className={`hb-streak${n > 0 ? " is-on" : ""}`}
      title={`Sequência atual: ${n} · melhor: ${habit.best_streak}`}
    >
      <IconFlame />
      {n}
      <span className="hb-streak-unit">{n === 1 ? " dia" : " dias"}</span>
    </span>
  );
}

function Ring({ done, total }: { done: number; total: number }) {
  const r = 26;
  const c = 2 * Math.PI * r;
  const ratio = total ? done / total : 0;
  return (
    <svg className="hb-ring" viewBox="0 0 64 64" aria-hidden="true">
      <circle cx="32" cy="32" r={r} className="hb-ring-track" />
      <circle
        cx="32"
        cy="32"
        r={r}
        className="hb-ring-fill"
        strokeDasharray={c}
        strokeDashoffset={c * (1 - ratio)}
        transform="rotate(-90 32 32)"
      />
    </svg>
  );
}

export function HabitsBoard({ habits, today, historyDays }: { habits: Habit[]; today: string; historyDays: number }) {
  const [editing, setEditing] = useState<Habit | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [, startTransition] = useTransition();

  const baseLogs = useMemo(() => Object.fromEntries(habits.map((h) => [h.id, h.logs])) as Logs, [habits]);
  const [logs, setOptimisticLog] = useOptimistic(baseLogs, (state: Logs, change: { id: string; day: string; count: number }) => ({
    ...state,
    [change.id]: { ...state[change.id], [change.day]: change.count },
  }));

  const active = habits.filter((h) => !h.archived);
  const archived = habits.filter((h) => h.archived);
  const countOn = (h: Habit, day: string) => logs[h.id]?.[day] ?? 0;
  const doneOn = (h: Habit, day: string) => countOn(h, day) >= h.target;

  const setCount = (h: Habit, day: string, count: number) => {
    if (day > today) return;
    setError(null);
    startTransition(async () => {
      setOptimisticLog({ id: h.id, day, count });
      const result = await setHabitLogAction(h.id, day, count);
      if (result.error) setError(result.error);
    });
  };
  const toggle = (h: Habit, day: string) => setCount(h, day, doneOn(h, day) ? 0 : h.target);

  const todays = active.filter((h) => scheduled(h, today));
  const others = active.filter((h) => !scheduled(h, today));
  const doneToday = todays.filter((h) => doneOn(h, today)).length;

  return (
    <>
      <div className="topbar">
        <h1 className="page-title">Hábitos</h1>
        <button type="button" className="btn-primary" onClick={() => setEditing("new")}>
          <IconPlus />
          Novo hábito
        </button>
      </div>
      {error && <p className="form-error">{error}</p>}

      {active.length === 0 ? (
        <section className="panel hb-empty">
          <p>Nenhum hábito ainda.</p>
          <p className="hb-muted">
            Comece por algo pequeno: ler 10 páginas, beber 8 copos de água, dormir antes da meia-noite.
          </p>
          <button type="button" className="btn-primary" onClick={() => setEditing("new")}>
            <IconPlus />
            Criar o primeiro hábito
          </button>
        </section>
      ) : (
        <>
          <section className="panel hb-panel hb-today">
            <header className="hb-today-head">
              <div className="hb-today-progress">
                <Ring done={doneToday} total={todays.length} />
                <b>
                  {doneToday}
                  <small>/{todays.length}</small>
                </b>
              </div>
              <div>
                <h2>Hoje</h2>
                <p className="hb-muted">
                  {dayLabel(today)} ·{" "}
                  {todays.length === 0
                    ? "nada marcado para hoje"
                    : doneToday === todays.length
                      ? "tudo feito"
                      : `falta${todays.length - doneToday === 1 ? "" : "m"} ${todays.length - doneToday}`}
                </p>
              </div>
            </header>

            <ul className="hb-rows">
              {todays.map((h) => (
                <HabitRow key={h.id} habit={h} count={countOn(h, today)} onToggle={() => toggle(h, today)} onCount={(n) => setCount(h, today, n)} onEdit={() => setEditing(h)} />
              ))}
            </ul>
            {others.length > 0 && (
              <>
                <h3 className="hb-subhead">Fora de hoje</h3>
                <ul className="hb-rows is-quiet">
                  {others.map((h) => (
                    <HabitRow key={h.id} habit={h} count={countOn(h, today)} onToggle={() => toggle(h, today)} onCount={(n) => setCount(h, today, n)} onEdit={() => setEditing(h)} />
                  ))}
                </ul>
              </>
            )}
          </section>

          <WeekGrid habits={active} today={today} countOn={countOn} doneOn={doneOn} onToggle={toggle} />
          <YearMap habits={active} today={today} historyDays={historyDays} countOn={countOn} doneOn={doneOn} />
        </>
      )}

      {archived.length > 0 && (
        <section className="panel hb-panel">
          <div className="panel-head">
            <h2>Arquivados</h2>
            <span>{archived.length}</span>
          </div>
          <ul className="hb-archived">
            {archived.map((h) => (
              <li key={h.id} style={{ "--hb": h.color } as CSSProperties}>
                <span className="hb-dot" />
                <span className="hb-archived-name">{h.name}</span>
                <span className="hb-muted">melhor sequência: {h.best_streak}</span>
                <button type="button" className="icon-btn" aria-label={`Editar ${h.name}`} onClick={() => setEditing(h)}>
                  <IconPencil />
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      <Modal open={editing !== null} onClose={() => setEditing(null)}>
        {editing !== null && (
          <HabitEditor key={editing === "new" ? "new" : editing.id} habit={editing === "new" ? null : editing} onClose={() => setEditing(null)} />
        )}
      </Modal>
    </>
  );
}

function HabitRow({
  habit: h,
  count,
  onToggle,
  onCount,
  onEdit,
}: {
  habit: Habit;
  count: number;
  onToggle: () => void;
  onCount: (n: number) => void;
  onEdit: () => void;
}) {
  const done = count >= h.target;
  return (
    <li className={`hb-row${done ? " is-done" : ""}`} style={{ "--hb": h.color } as CSSProperties}>
      {h.kind === "check" ? (
        <button type="button" className="hb-check" aria-pressed={done} aria-label={done ? `Desmarcar ${h.name}` : `Marcar ${h.name}`} onClick={onToggle}>
          <IconCheck />
        </button>
      ) : (
        <button type="button" className="hb-check hb-check-count" aria-label={done ? `Zerar ${h.name}` : `Completar ${h.name}`} onClick={onToggle}>
          {done ? <IconCheck /> : <span>{count}</span>}
        </button>
      )}

      <div className="hb-row-main">
        <button type="button" className="hb-row-name" title="Editar hábito" onClick={onEdit}>
          {h.name}
        </button>
        {h.kind === "count" ? (
          <div className="hb-count">
            <div className="hb-bar">
              <span style={{ width: `${Math.min(100, (count / h.target) * 100)}%` }} />
            </div>
            <span className="hb-count-label">
              {count} de {h.target}
              {h.unit ? ` ${h.unit}` : ""}
            </span>
          </div>
        ) : (
          <span className="hb-muted">{daysLabel(h.days)}</span>
        )}
      </div>

      {h.kind === "count" && (
        <div className="hb-stepper">
          <button type="button" aria-label={`Menos ${h.unit || "um"}`} disabled={count === 0} onClick={() => onCount(count - 1)}>
            −
          </button>
          <button type="button" aria-label={`Mais ${h.unit || "um"}`} onClick={() => onCount(count + 1)}>
            +
          </button>
        </div>
      )}
      <Streak habit={h} />
      <button type="button" className="icon-btn" aria-label={`Editar ${h.name}`} onClick={onEdit}>
        <IconPencil />
      </button>
    </li>
  );
}

function WeekGrid({
  habits,
  today,
  countOn,
  doneOn,
  onToggle,
}: {
  habits: Habit[];
  today: string;
  countOn: (h: Habit, day: string) => number;
  doneOn: (h: Habit, day: string) => boolean;
  onToggle: (h: Habit, day: string) => void;
}) {
  const days = Array.from({ length: 7 }, (_, i) => shiftDay(today, i - 6));
  // On a narrow screen the table scrolls; start at today's end of it.
  const scroller = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = scroller.current;
    if (el) el.scrollLeft = el.scrollWidth;
  }, []);
  return (
    <section className="panel hb-panel">
      <div className="panel-head">
        <h2>Últimos 7 dias</h2>
        <span className="hb-hint">Toque num dia para marcar ou desmarcar</span>
      </div>
      <div className="hb-week-scroll" ref={scroller}>
        <table className="hb-week">
          <thead>
            <tr>
              <th />
              {days.map((d) => (
                <th key={d} className={d === today ? "is-today" : ""}>
                  <span>{DAY_SHORT[weekdayOf(d)]}</span>
                  <b>{Number(d.slice(8))}</b>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {habits.map((h) => (
              <tr key={h.id} style={{ "--hb": h.color } as CSSProperties}>
                <th scope="row">{h.name}</th>
                {days.map((d) => {
                  const on = scheduled(h, d);
                  const done = doneOn(h, d);
                  const partial = !done && countOn(h, d) > 0;
                  const label = `${h.name}, ${d}: ${done ? "feito" : partial ? `${countOn(h, d)} de ${h.target}` : "não feito"}`;
                  return (
                    <td key={d}>
                      <button
                        type="button"
                        className={`hb-cell${done ? " is-done" : partial ? " is-partial" : ""}${on ? "" : " is-off"}`}
                        aria-label={label}
                        title={label}
                        aria-pressed={done}
                        onClick={() => onToggle(h, d)}
                      >
                        {done && <IconCheck />}
                      </button>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function YearMap({
  habits,
  today,
  historyDays,
  countOn,
  doneOn,
}: {
  habits: Habit[];
  today: string;
  historyDays: number;
  countOn: (h: Habit, day: string) => number;
  doneOn: (h: Habit, day: string) => boolean;
}) {
  const [selected, setSelected] = useState<string>("all");
  const habit = habits.find((h) => h.id === selected) ?? null;
  const pool = habit ? [habit] : habits;

  // How much of a day got done, 0..1; null when nothing was due that day.
  const valueOf = (day: string): number | null => {
    const due = pool.filter((h) => scheduled(h, day));
    if (due.length === 0) return null;
    if (habit) return Math.min(1, countOn(habit, day) / habit.target);
    return due.filter((h) => doneOn(h, day)).length / due.length;
  };

  // Columns are weeks, Monday first; the last column holds today.
  const offset = (weekdayOf(today) + 6) % 7;
  const weeks = Math.floor((historyDays - offset - 1) / 7) + 1;
  const firstMonday = shiftDay(today, -offset - (weeks - 1) * 7);
  const columns = Array.from({ length: weeks }, (_, w) =>
    WEEK_ORDER.map((_, r) => shiftDay(firstMonday, w * 7 + r)),
  );

  const last30 = Array.from({ length: 30 }, (_, i) => valueOf(shiftDay(today, -i))).filter((v): v is number => v !== null);
  const rate = last30.length ? Math.round((last30.reduce((a, b) => a + b, 0) / last30.length) * 100) : null;
  const best = habit ? habit.best_streak : Math.max(0, ...habits.map((h) => h.best_streak));
  const yearScroller = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = yearScroller.current;
    if (el) el.scrollLeft = el.scrollWidth;
  }, []);

  return (
    <section className="panel hb-panel">
      <div className="panel-head hb-year-head">
        <h2>O ano</h2>
        <select value={selected} onChange={(e) => setSelected(e.target.value)} aria-label="Hábito">
          <option value="all">Todos os hábitos</option>
          {habits.map((h) => (
            <option key={h.id} value={h.id}>
              {h.name}
            </option>
          ))}
        </select>
      </div>

      <div className="hb-year-stats">
        <div>
          <b>{rate === null ? "—" : `${rate}%`}</b>
          <span>feito nos últimos 30 dias</span>
        </div>
        <div>
          <b>{best}</b>
          <span>{best === 1 ? "dia na melhor sequência" : "dias na melhor sequência"}</span>
        </div>
      </div>

      <div className="hb-year-scroll" ref={yearScroller}>
        <div
          className="hb-year"
          style={{ "--hb": habit?.color ?? "var(--m-habitos)", "--weeks": weeks } as CSSProperties}
          role="img"
          aria-label="Mapa do ano: quanto foi feito em cada dia"
        >
          <span />
          {columns.map((col, w) => {
            const first = col.find((d) => d.slice(8) === "01");
            return (
              <span key={`m${w}`} className="hb-year-month">
                {first ? MONTHS[Number(first.slice(5, 7)) - 1] : ""}
              </span>
            );
          })}
          {WEEK_ORDER.map((weekday, r) => (
            <Fragment key={weekday}>
              <span className="hb-year-day">{r % 2 === 0 ? DAY_SHORT[weekday] : ""}</span>
              {columns.map((col) => {
                const day = col[r];
                if (day > today) return <span key={day} className="hb-sq is-future" />;
                const v = valueOf(day);
                return (
                  <span
                    key={day}
                    className={`hb-sq${v === null ? " is-none" : ""}`}
                    style={v ? ({ "--v": v } as CSSProperties) : undefined}
                    title={`${day}: ${v === null ? "nada previsto" : `${Math.round(v * 100)}%`}`}
                  />
                );
              })}
            </Fragment>
          ))}
        </div>
      </div>
      <div className="hb-legend" aria-hidden="true">
        Menos
        {[0, 0.25, 0.5, 0.75, 1].map((v) => (
          <span key={v} className="hb-sq" style={{ "--v": v } as CSSProperties} />
        ))}
        Mais
      </div>
    </section>
  );
}

function HabitEditor({ habit, onClose }: { habit: Habit | null; onClose: () => void }) {
  const [name, setName] = useState(habit?.name ?? "");
  const [kind, setKind] = useState<HabitKind>(habit?.kind ?? "check");
  const [target, setTarget] = useState(String(habit?.kind === "count" ? habit.target : 8));
  const [unit, setUnit] = useState(habit?.unit ?? "");
  const [days, setDays] = useState<number[]>(habit?.days ?? [0, 1, 2, 3, 4, 5, 6]);
  const [color, setColor] = useState(habit?.color ?? COLORS[0]);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const input = (archived = habit?.archived ?? false): HabitInput => ({
    name,
    kind,
    target: kind === "count" ? Number(target) : 1,
    unit: kind === "count" ? unit : "",
    days,
    color,
    archived,
  });

  const run = (fn: () => Promise<{ error?: string }>) => {
    setError(null);
    startTransition(async () => {
      const result = await fn();
      if (result.error) setError(result.error);
      else onClose();
    });
  };

  return (
    <div className="panel" style={{ "--mod": color } as CSSProperties}>
      <div className="panel-head">
        <h2>{habit ? "Editar hábito" : "Novo hábito"}</h2>
      </div>
      <form
        className="form-grid"
        onSubmit={(e) => {
          e.preventDefault();
          run(() => saveHabitAction(input(), habit?.id));
        }}
      >
        <div className="field">
          <label htmlFor="hb-name">Nome</label>
          <input id="hb-name" value={name} maxLength={80} placeholder="Ler 10 páginas" autoFocus onChange={(e) => setName(e.target.value)} />
        </div>

        <div className="field">
          <label>Como marcar</label>
          <div className="seg" role="group" aria-label="Como marcar">
            <button type="button" className={kind === "check" ? "active" : ""} aria-pressed={kind === "check"} onClick={() => setKind("check")}>
              Feito ou não
            </button>
            <button type="button" className={kind === "count" ? "active" : ""} aria-pressed={kind === "count"} onClick={() => setKind("count")}>
              Contar até uma meta
            </button>
          </div>
        </div>

        {kind === "count" && (
          <div className="row2">
            <div className="field">
              <label htmlFor="hb-target">Meta por dia</label>
              <input id="hb-target" type="number" min={1} max={1000} value={target} onChange={(e) => setTarget(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="hb-unit">Unidade</label>
              <input id="hb-unit" value={unit} maxLength={20} placeholder="copos" onChange={(e) => setUnit(e.target.value)} />
            </div>
          </div>
        )}

        <div className="field">
          <label>Dias</label>
          <DayPicker value={days} onChange={setDays} />
        </div>

        <div className="field">
          <label>Cor</label>
          <div className="hb-colors" role="radiogroup" aria-label="Cor">
            {COLORS.map((c) => (
              <button
                key={c}
                type="button"
                role="radio"
                aria-checked={color === c}
                aria-label={c}
                className={color === c ? "is-on" : ""}
                style={{ "--hb": c } as CSSProperties}
                onClick={() => setColor(c)}
              />
            ))}
          </div>
        </div>

        {error && <p className="form-error">{error}</p>}

        <div className="hb-editor-actions">
          <button type="submit" className="btn-primary" disabled={pending}>
            {pending ? "Salvando…" : habit ? "Salvar" : "Criar hábito"}
          </button>
          {habit && (
            <>
              <button type="button" className="btn-outline" disabled={pending} onClick={() => run(() => saveHabitAction(input(!habit.archived), habit.id))}>
                {habit.archived ? "Restaurar" : "Arquivar"}
              </button>
              <button
                type="button"
                className="btn-outline bad"
                disabled={pending}
                onClick={() => {
                  if (window.confirm(`Excluir "${habit.name}" e todo o histórico dele? Para só parar de acompanhar, arquive.`)) {
                    run(() => deleteHabitAction(habit.id));
                  }
                }}
              >
                Excluir
              </button>
            </>
          )}
        </div>
      </form>
    </div>
  );
}
