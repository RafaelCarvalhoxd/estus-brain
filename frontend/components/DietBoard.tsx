"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { DietTargets, Meal } from "@/lib/diet";
import { deleteMealAction, saveMealAction, saveTargetsAction } from "@/app/dieta/actions";
import { formatAmount, sumMacros } from "@/lib/macros";
import { DAY_LONG, DAY_SHORT, WEEK_ORDER, capitalize, daysLabel, plural, timeToMinutes } from "@/lib/week";
import { DayPicker } from "./DayPicker";
import { Modal } from "./Modal";
import { IconPencil, IconPlus, IconTrash } from "./icons";

const byTime = (a: Meal, b: Meal) => a.time.localeCompare(b.time);

// Decimal input typed the Brazilian way ("12,5") or not.
const num = (s: string) => {
  const n = Number(s.replace(",", "."));
  return Number.isFinite(n) ? n : NaN;
};

function MacroStat({ label, value, target, unit, digits = 0 }: { label: string; value: number; target: number; unit: string; digits?: number }) {
  const pct = target > 0 ? Math.round((value / target) * 100) : null;
  return (
    <div className="macro">
      <span className="macro-label">{label}</span>
      <b>
        {formatAmount(value, digits)}
        <small>{unit}</small>
      </b>
      {pct !== null ? (
        <>
          <span className="macro-of">
            meta {formatAmount(target, digits)} {unit}, {pct}%
          </span>
          <span className="macro-bar" aria-hidden="true">
            <i className={value > target * 1.05 ? "over" : ""} style={{ width: `${Math.min(100, pct)}%` }} />
          </span>
        </>
      ) : (
        <span className="macro-of">sem meta</span>
      )}
    </div>
  );
}

type ItemDraft = { food: string; quantity: string; kcal: string; protein: string; carbs: string; fat: string };
const blankItem = (): ItemDraft => ({ food: "", quantity: "", kcal: "", protein: "", carbs: "", fat: "" });

function MealEditor({ meal, onClose }: { meal: Meal | null; onClose: () => void }) {
  const router = useRouter();
  const [name, setName] = useState(meal?.name ?? "");
  const [time, setTime] = useState(meal?.time ?? "12:00");
  const [days, setDays] = useState<number[]>(meal?.days ?? [0, 1, 2, 3, 4, 5, 6]);
  const [notes, setNotes] = useState(meal?.notes ?? "");
  const [rows, setRows] = useState<ItemDraft[]>(
    meal?.items.length
      ? meal.items.map((it) => ({
          food: it.food,
          quantity: it.quantity,
          kcal: String(it.kcal),
          protein: String(it.protein_g),
          carbs: String(it.carbs_g),
          fat: String(it.fat_g),
        }))
      : [blankItem()],
  );
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [pending, startTransition] = useTransition();

  const update = (i: number, patch: Partial<ItemDraft>) =>
    setRows((list) => list.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  const items = rows.map((r) => ({
    food: r.food,
    quantity: r.quantity,
    kcal: r.kcal === "" ? 0 : num(r.kcal),
    protein_g: r.protein === "" ? 0 : num(r.protein),
    carbs_g: r.carbs === "" ? 0 : num(r.carbs),
    fat_g: r.fat === "" ? 0 : num(r.fat),
  }));
  const totals = sumMacros(items.filter((it) => it.food.trim()).map((it) => ({
    kcal: it.kcal || 0,
    protein_g: it.protein_g || 0,
    carbs_g: it.carbs_g || 0,
    fat_g: it.fat_g || 0,
  })));

  const save = () => {
    setError(null);
    startTransition(async () => {
      const result = await saveMealAction({ name, time, days, notes, items }, meal?.id);
      if (result.error) {
        setError(result.error);
        return;
      }
      router.refresh();
      onClose();
    });
  };

  const remove = () => {
    if (!meal) return;
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    startTransition(async () => {
      const result = await deleteMealAction(meal.id);
      if (result.error) {
        setError(result.error);
        return;
      }
      router.refresh();
      onClose();
    });
  };

  return (
    <div className="panel plan-editor">
      <div className="panel-head">
        <h2>{meal ? "Editar refeição" : "Nova refeição"}</h2>
      </div>

      <div className="row2">
        <div className="field">
          <label htmlFor="m-name">Nome</label>
          <input id="m-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Almoço" maxLength={120} />
        </div>
        <div className="field">
          <label htmlFor="m-time">Horário</label>
          <input id="m-time" type="time" value={time} onChange={(e) => setTime(e.target.value)} />
        </div>
      </div>

      <div className="field">
        <label>Dias da semana</label>
        <DayPicker value={days} onChange={setDays} />
      </div>

      <div className="field">
        <label>Alimentos</label>
        <div className="lines-scroll">
          <div className="lines">
            <div className="line-row food line-head" aria-hidden="true">
              <span>Alimento</span>
              <span>Quantidade</span>
              <span>kcal</span>
              <span>Prot. (g)</span>
              <span>Carb. (g)</span>
              <span>Gord. (g)</span>
              <span />
            </div>
            {rows.map((row, i) => (
              <div className="line-row food" key={i}>
                <input aria-label="Alimento" value={row.food} onChange={(e) => update(i, { food: e.target.value })} placeholder="Arroz branco" maxLength={120} />
                <input aria-label="Quantidade" value={row.quantity} onChange={(e) => update(i, { quantity: e.target.value })} placeholder="150 g" maxLength={40} />
                <input aria-label="Calorias" inputMode="decimal" value={row.kcal} onChange={(e) => update(i, { kcal: e.target.value })} placeholder="0" />
                <input aria-label="Proteína em gramas" inputMode="decimal" value={row.protein} onChange={(e) => update(i, { protein: e.target.value })} placeholder="0" />
                <input aria-label="Carboidratos em gramas" inputMode="decimal" value={row.carbs} onChange={(e) => update(i, { carbs: e.target.value })} placeholder="0" />
                <input aria-label="Gorduras em gramas" inputMode="decimal" value={row.fat} onChange={(e) => update(i, { fat: e.target.value })} placeholder="0" />
                <button type="button" className="icon-btn bad" aria-label="Remover alimento" onClick={() => setRows((list) => list.filter((_, j) => j !== i))}>
                  <IconTrash />
                </button>
              </div>
            ))}
          </div>
        </div>
        <div className="line-foot">
          <button type="button" className="btn-outline line-add" onClick={() => setRows((list) => [...list, blankItem()])}>
            <IconPlus />
            Adicionar alimento
          </button>
          <span className="line-totals">
            Total: {formatAmount(totals.kcal)} kcal, P {formatAmount(totals.protein_g, 1)} g, C {formatAmount(totals.carbs_g, 1)} g, G{" "}
            {formatAmount(totals.fat_g, 1)} g
          </span>
        </div>
      </div>

      <div className="field">
        <label htmlFor="m-notes">Observações (opcional)</label>
        <textarea id="m-notes" value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Substituições, modo de preparo" maxLength={2000} />
      </div>

      {error && <p className="form-error">{error}</p>}

      <div className="plan-editor-foot">
        <div>
          {meal && (
            <button type="button" className="btn-text bad" onClick={remove} disabled={pending}>
              {confirmDelete ? "Confirmar exclusão" : "Excluir refeição"}
            </button>
          )}
        </div>
        <div className="plan-editor-actions">
          <button type="button" className="btn-outline" onClick={onClose} disabled={pending}>
            Cancelar
          </button>
          <button type="button" className="btn-primary" onClick={save} disabled={pending}>
            {pending ? "Salvando…" : "Salvar refeição"}
          </button>
        </div>
      </div>
    </div>
  );
}

function TargetsForm({ targets }: { targets: DietTargets }) {
  const router = useRouter();
  const show = (n: number) => (n ? String(n) : "");
  const [kcal, setKcal] = useState(show(targets.kcal));
  const [protein, setProtein] = useState(show(targets.protein_g));
  const [carbs, setCarbs] = useState(show(targets.carbs_g));
  const [fat, setFat] = useState(show(targets.fat_g));
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);
  const [pending, startTransition] = useTransition();

  const save = () => {
    setMessage(null);
    const value = (s: string) => (s.trim() === "" ? 0 : num(s));
    startTransition(async () => {
      const result = await saveTargetsAction({ kcal: value(kcal), protein_g: value(protein), carbs_g: value(carbs), fat_g: value(fat) });
      if (result.error) {
        setMessage({ ok: false, text: result.error });
        return;
      }
      setMessage({ ok: true, text: "Metas salvas." });
      router.refresh();
    });
  };

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Metas diárias</h2>
        <span>Deixe em branco o que não quiser acompanhar</span>
      </div>
      <div className="targets-form">
        <div className="field">
          <label htmlFor="t-kcal">Calorias (kcal)</label>
          <input id="t-kcal" inputMode="decimal" value={kcal} onChange={(e) => setKcal(e.target.value)} placeholder="2200" />
        </div>
        <div className="field">
          <label htmlFor="t-protein">Proteína (g)</label>
          <input id="t-protein" inputMode="decimal" value={protein} onChange={(e) => setProtein(e.target.value)} placeholder="160" />
        </div>
        <div className="field">
          <label htmlFor="t-carbs">Carboidratos (g)</label>
          <input id="t-carbs" inputMode="decimal" value={carbs} onChange={(e) => setCarbs(e.target.value)} placeholder="250" />
        </div>
        <div className="field">
          <label htmlFor="t-fat">Gorduras (g)</label>
          <input id="t-fat" inputMode="decimal" value={fat} onChange={(e) => setFat(e.target.value)} placeholder="70" />
        </div>
        <button type="button" className="btn-primary" onClick={save} disabled={pending}>
          {pending ? "Salvando…" : "Salvar metas"}
        </button>
      </div>
      {message && <p className={message.ok ? "form-success" : "form-error"}>{message.text}</p>}
    </section>
  );
}

export function DietBoard({ meals, targets, today, nowMinutes }: { meals: Meal[]; targets: DietTargets; today: number; nowMinutes: number }) {
  const [editing, setEditing] = useState<Meal | "new" | null>(null);

  const onDay = (d: number) => meals.filter((m) => m.days.includes(d)).sort(byTime);
  const todays = onDay(today);
  const todayTotals = sumMacros(todays.flatMap((m) => m.items));

  // The meal happening now is the last one whose time has come; the next is
  // the one after it.
  let current = -1;
  todays.forEach((m, i) => {
    if (timeToMinutes(m.time) <= nowMinutes) current = i;
  });
  const nextIdx = current + 1 < todays.length ? current + 1 : -1;

  return (
    <>
      <div className="topbar">
        <h1 className="page-title">Dieta</h1>
        <button type="button" className="btn-primary" onClick={() => setEditing("new")}>
          <IconPlus />
          Nova refeição
        </button>
      </div>

      <section className="panel plan-today">
        <div className="plan-today-head">
          <h2>Hoje</h2>
          <span>{capitalize(DAY_LONG[today])}</span>
        </div>

        <div className="macro-grid">
          <MacroStat label="Calorias" value={todayTotals.kcal} target={targets.kcal} unit="kcal" />
          <MacroStat label="Proteína" value={todayTotals.protein_g} target={targets.protein_g} unit="g" digits={1} />
          <MacroStat label="Carboidratos" value={todayTotals.carbs_g} target={targets.carbs_g} unit="g" digits={1} />
          <MacroStat label="Gorduras" value={todayTotals.fat_g} target={targets.fat_g} unit="g" digits={1} />
        </div>

        {meals.length === 0 ? (
          <div className="plan-empty">
            <p>Nenhuma refeição cadastrada ainda. Monte o seu plano com os alimentos e os macros de cada refeição.</p>
            <button type="button" className="btn-outline" onClick={() => setEditing("new")}>
              Cadastrar refeição
            </button>
          </div>
        ) : todays.length === 0 ? (
          <p className="plan-muted meal-none">Nenhuma refeição planejada pra hoje.</p>
        ) : (
          <div className="meal-list">
            {todays.map((m, i) => {
              const t = sumMacros(m.items);
              return (
                <div key={m.id} className={`meal-row${i === current ? " is-now" : ""}`}>
                  <span className="meal-time">{m.time}</span>
                  <div className="meal-main">
                    <div className="meal-name">
                      <b>{m.name}</b>
                      {i === current && <span className="meal-badge">Agora</span>}
                      {i === nextIdx && <span className="meal-badge is-next">Próxima</span>}
                    </div>
                    <div className="meal-foods">
                      {m.items.length
                        ? m.items.map((it) => (it.quantity ? `${it.food} (${it.quantity})` : it.food)).join(", ")
                        : "Sem alimentos ainda"}
                    </div>
                  </div>
                  <div className="meal-macros">
                    <b>{formatAmount(t.kcal)} kcal</b>
                    <span>
                      P {formatAmount(t.protein_g, 1)} g, C {formatAmount(t.carbs_g, 1)} g, G {formatAmount(t.fat_g, 1)} g
                    </span>
                    <button type="button" className="btn-text" onClick={() => setEditing(m)}>
                      Editar
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section className="panel plan-week">
        <div className="panel-head">
          <h2>Semana</h2>
        </div>
        <div className="week-scroll">
          <div className="week-grid">
            {WEEK_ORDER.map((d) => {
              const list = onDay(d);
              const t = sumMacros(list.flatMap((m) => m.items));
              return (
                <div key={d} className={`week-col${d === today ? " is-today" : ""}`}>
                  <div className="week-col-head">
                    {DAY_SHORT[d]}
                    {d === today && <em>hoje</em>}
                  </div>
                  {list.length ? (
                    list.map((m) => (
                      <button key={m.id} type="button" className="week-chip" onClick={() => setEditing(m)}>
                        <b>{m.name}</b>
                        <span>{m.time}</span>
                      </button>
                    ))
                  ) : (
                    <span className="week-rest">Sem plano</span>
                  )}
                  {list.length > 0 && (
                    <div className="week-total">
                      <b>{formatAmount(t.kcal)} kcal</b>
                      <span>
                        P {formatAmount(t.protein_g)} C {formatAmount(t.carbs_g)} G {formatAmount(t.fat_g)}
                      </span>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <TargetsForm targets={targets} />

      {meals.length > 0 && (
        <section className="panel">
          <div className="panel-head">
            <h2>Suas refeições</h2>
            <span>{plural(meals.length, "refeição", "refeições")}</span>
          </div>
          <div className="plan-list">
            {[...meals].sort(byTime).map((m) => {
              const t = sumMacros(m.items);
              return (
                <div className="plan-list-row" key={m.id}>
                  <span className="meal-time">{m.time}</span>
                  <div className="plan-list-main">
                    <b>{m.name}</b>
                    <span>
                      {daysLabel(m.days)}, {formatAmount(t.kcal)} kcal, {plural(m.items.length, "alimento", "alimentos")}
                    </span>
                  </div>
                  <div className="row-actions">
                    <button type="button" className="icon-btn" aria-label={`Editar ${m.name}`} onClick={() => setEditing(m)}>
                      <IconPencil />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      <Modal open={editing !== null} onClose={() => setEditing(null)} wide>
        {editing !== null && (
          <MealEditor key={editing === "new" ? "new" : editing.id} meal={editing === "new" ? null : editing} onClose={() => setEditing(null)} />
        )}
      </Modal>
    </>
  );
}
