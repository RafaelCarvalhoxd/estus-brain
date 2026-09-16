"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Workout } from "@/lib/training";
import { deleteWorkoutAction, saveWorkoutAction } from "@/app/treino/actions";
import { DAY_LONG, DAY_SHORT, WEEK_ORDER, capitalize, daysLabel, plural } from "@/lib/week";
import { DayPicker } from "./DayPicker";
import { Modal } from "./Modal";
import { IconPencil, IconPlus, IconTrash } from "./icons";

function formatRest(seconds: number): string {
  if (!seconds) return "—";
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return s ? `${m}min ${s}s` : `${m}min`;
}

// Editor rows hold what's typed, as text, so a half-typed number isn't
// coerced while you're still typing it.
type ExerciseDraft = { name: string; sets: string; reps: string; weight: string; rest: string; notes: string };
const blankExercise = (): ExerciseDraft => ({ name: "", sets: "3", reps: "10", weight: "", rest: "60", notes: "" });

function WorkoutEditor({ workout, defaultDay, onClose }: { workout: Workout | null; defaultDay: number; onClose: () => void }) {
  const router = useRouter();
  const [name, setName] = useState(workout?.name ?? "");
  const [focus, setFocus] = useState(workout?.focus ?? "");
  const [days, setDays] = useState<number[]>(workout?.days ?? [defaultDay]);
  const [notes, setNotes] = useState(workout?.notes ?? "");
  const [rows, setRows] = useState<ExerciseDraft[]>(
    workout?.exercises.length
      ? workout.exercises.map((e) => ({
          name: e.name,
          sets: String(e.sets),
          reps: e.reps,
          weight: e.weight,
          rest: String(e.rest_seconds),
          notes: e.notes,
        }))
      : [blankExercise()],
  );
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [pending, startTransition] = useTransition();

  const update = (i: number, patch: Partial<ExerciseDraft>) =>
    setRows((list) => list.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  const save = () => {
    setError(null);
    startTransition(async () => {
      const result = await saveWorkoutAction(
        {
          name,
          focus,
          days,
          notes,
          exercises: rows.map((r) => ({
            name: r.name,
            sets: Number(r.sets),
            reps: r.reps,
            weight: r.weight,
            rest_seconds: Math.max(0, Math.round(Number(r.rest) || 0)),
            notes: r.notes,
          })),
        },
        workout?.id,
      );
      if (result.error) {
        setError(result.error);
        return;
      }
      router.refresh();
      onClose();
    });
  };

  const remove = () => {
    if (!workout) return;
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    startTransition(async () => {
      const result = await deleteWorkoutAction(workout.id);
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
        <h2>{workout ? "Editar treino" : "Novo treino"}</h2>
      </div>

      <div className="row2">
        <div className="field">
          <label htmlFor="w-name">Nome</label>
          <input id="w-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Treino A" maxLength={120} />
        </div>
        <div className="field">
          <label htmlFor="w-focus">Foco (opcional)</label>
          <input id="w-focus" value={focus} onChange={(e) => setFocus(e.target.value)} placeholder="Peito e tríceps" maxLength={120} />
        </div>
      </div>

      <div className="field">
        <label>Dias da semana</label>
        <DayPicker value={days} onChange={setDays} />
      </div>

      <div className="field">
        <label>Exercícios</label>
        <div className="lines-scroll">
          <div className="lines">
            <div className="line-row exercise line-head" aria-hidden="true">
              <span>Exercício</span>
              <span>Séries</span>
              <span>Reps</span>
              <span>Carga</span>
              <span>Descanso (s)</span>
              <span />
            </div>
            {rows.map((row, i) => (
              <div className="line-row exercise" key={i}>
                <input aria-label="Exercício" value={row.name} onChange={(e) => update(i, { name: e.target.value })} placeholder="Supino reto" maxLength={120} />
                <input aria-label="Séries" type="number" min={1} max={50} value={row.sets} onChange={(e) => update(i, { sets: e.target.value })} />
                <input aria-label="Repetições" value={row.reps} onChange={(e) => update(i, { reps: e.target.value })} placeholder="8-12" maxLength={20} />
                <input aria-label="Carga" value={row.weight} onChange={(e) => update(i, { weight: e.target.value })} placeholder="20 kg" maxLength={30} />
                <input aria-label="Descanso em segundos" type="number" min={0} max={3600} step={15} value={row.rest} onChange={(e) => update(i, { rest: e.target.value })} />
                <button type="button" className="icon-btn bad" aria-label="Remover exercício" onClick={() => setRows((list) => list.filter((_, j) => j !== i))}>
                  <IconTrash />
                </button>
              </div>
            ))}
          </div>
        </div>
        <button type="button" className="btn-outline line-add" onClick={() => setRows((list) => [...list, blankExercise()])}>
          <IconPlus />
          Adicionar exercício
        </button>
      </div>

      <div className="field">
        <label htmlFor="w-notes">Observações (opcional)</label>
        <textarea id="w-notes" value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Aquecimento, cadência, o que for útil lembrar" maxLength={2000} />
      </div>

      {error && <p className="form-error">{error}</p>}

      <div className="plan-editor-foot">
        <div>
          {workout && (
            <button type="button" className="btn-text bad" onClick={remove} disabled={pending}>
              {confirmDelete ? "Confirmar exclusão" : "Excluir treino"}
            </button>
          )}
        </div>
        <div className="plan-editor-actions">
          <button type="button" className="btn-outline" onClick={onClose} disabled={pending}>
            Cancelar
          </button>
          <button type="button" className="btn-primary" onClick={save} disabled={pending}>
            {pending ? "Salvando…" : "Salvar treino"}
          </button>
        </div>
      </div>
    </div>
  );
}

export function TrainingBoard({ workouts, today }: { workouts: Workout[]; today: number }) {
  const [editing, setEditing] = useState<Workout | "new" | null>(null);

  const onDay = (d: number) => workouts.filter((w) => w.days.includes(d));
  const todays = onDay(today);
  let next: { day: number; workout: Workout } | null = null;
  if (todays.length === 0) {
    for (let i = 1; i <= 7; i++) {
      const d = (today + i) % 7;
      const list = onDay(d);
      if (list.length) {
        next = { day: d, workout: list[0] };
        break;
      }
    }
  }

  return (
    <>
      <div className="topbar">
        <h1 className="page-title">Treino</h1>
        <button type="button" className="btn-primary" onClick={() => setEditing("new")}>
          <IconPlus />
          Novo treino
        </button>
      </div>

      <section className="panel plan-today">
        <div className="plan-today-head">
          <h2>Treino de hoje</h2>
          <span>{capitalize(DAY_LONG[today])}</span>
        </div>

        {workouts.length === 0 ? (
          <div className="plan-empty">
            <p>Nenhum treino cadastrado ainda. Monte o primeiro com os exercícios que você vai seguir.</p>
            <button type="button" className="btn-outline" onClick={() => setEditing("new")}>
              Cadastrar treino
            </button>
          </div>
        ) : todays.length === 0 ? (
          <div className="plan-empty">
            <p>
              Dia de descanso.
              {next && ` Próximo treino: ${next.workout.name}, ${DAY_LONG[next.day]}.`}
            </p>
          </div>
        ) : (
          todays.map((w) => (
            <div className="plan-block" key={w.id}>
              <div className="plan-block-title">
                <b>{w.name}</b>
                {w.focus && <span>{w.focus}</span>}
                <button type="button" className="btn-text" onClick={() => setEditing(w)}>
                  Editar
                </button>
              </div>
              {w.exercises.length === 0 ? (
                <p className="plan-muted">Sem exercícios ainda.</p>
              ) : (
                <div className="tbl-wrap">
                  <table className="plan-table">
                    <thead>
                      <tr>
                        <th>Exercício</th>
                        <th>Séries</th>
                        <th>Reps</th>
                        <th>Carga</th>
                        <th>Descanso</th>
                      </tr>
                    </thead>
                    <tbody>
                      {w.exercises.map((e, i) => (
                        <tr key={e.id ?? i}>
                          <td>{e.name}</td>
                          <td>{e.sets}</td>
                          <td>{e.reps || "—"}</td>
                          <td>{e.weight || "—"}</td>
                          <td>{formatRest(e.rest_seconds)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              {w.notes && <p className="plan-notes">{w.notes}</p>}
            </div>
          ))
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
              return (
                <div key={d} className={`week-col${d === today ? " is-today" : ""}`}>
                  <div className="week-col-head">
                    {DAY_SHORT[d]}
                    {d === today && <em>hoje</em>}
                  </div>
                  {list.length ? (
                    list.map((w) => (
                      <button key={w.id} type="button" className="week-chip" onClick={() => setEditing(w)}>
                        <b>{w.name}</b>
                        <span>{w.focus || plural(w.exercises.length, "exercício", "exercícios")}</span>
                      </button>
                    ))
                  ) : (
                    <span className="week-rest">Descanso</span>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </section>

      {workouts.length > 0 && (
        <section className="panel">
          <div className="panel-head">
            <h2>Seus treinos</h2>
            <span>{plural(workouts.length, "treino", "treinos")}</span>
          </div>
          <div className="plan-list">
            {workouts.map((w) => (
              <div className="plan-list-row" key={w.id}>
                <div className="plan-list-main">
                  <b>{w.name}</b>
                  <span>
                    {[w.focus, daysLabel(w.days), plural(w.exercises.length, "exercício", "exercícios")].filter(Boolean).join(", ")}
                  </span>
                </div>
                <div className="row-actions">
                  <button type="button" className="icon-btn" aria-label={`Editar ${w.name}`} onClick={() => setEditing(w)}>
                    <IconPencil />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <Modal open={editing !== null} onClose={() => setEditing(null)} wide>
        {editing !== null && (
          <WorkoutEditor
            key={editing === "new" ? "new" : editing.id}
            workout={editing === "new" ? null : editing}
            defaultDay={today}
            onClose={() => setEditing(null)}
          />
        )}
      </Modal>
    </>
  );
}
