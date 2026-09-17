import type { CSSProperties, ReactNode } from "react";
import Link from "next/link";
import {
  IconBell,
  IconCalendar,
  IconCard,
  IconDumbbell,
  IconFlow,
  IconFolder,
  IconHabit,
  IconLock,
  IconNote,
  IconUtensils,
  IconWallet,
} from "./icons";

// The overview behind the home screen's eye: one card per module, each
// showing what matters today and linking to the module. Everything is
// computed on the server; a section whose module didn't answer is null and
// says so instead of pretending to be empty.

export interface DashData {
  finance: { saldo: string; saldoCents: number; entradas: string; saidas: string; month: string } | null;
  bills: {
    payable: string;
    receivable: string;
    overdue: number;
    upcoming: { id: string; description: string; amount: string; due: string; receivable: boolean; late: boolean }[];
  } | null;
  training: {
    total: number;
    today: { id: string; name: string; focus: string; exercises: { name: string; detail: string }[] }[];
    next: { name: string; day: string } | null;
  } | null;
  diet: {
    total: number;
    meals: number;
    kcal: number;
    macros: { label: string; value: number; target: number }[];
    kcalTarget: number;
    now: { name: string; time: string } | null;
    next: { name: string; time: string } | null;
  } | null;
  agenda: { id: string; title: string; when: string; location: string }[] | null;
  reminders: { pending: number; items: { id: string; title: string; when: string; late: boolean }[] } | null;
  notes: { id: string; title: string; excerpt: string; when: string }[] | null;
  documents: { files: number; folders: number } | null;
  vault: { count: number } | null;
  boards: { total: number; recent: { id: string; name: string; preview: string; when: string }[] } | null;
  habits: { total: number; today: { id: string; name: string; color: string; done: boolean; progress: string }[] } | null;
}

const fmt = (n: number, digits = 0) => n.toLocaleString("pt-BR", { maximumFractionDigits: digits });

function Card({
  i,
  mod,
  icon,
  title,
  href,
  wide = false,
  children,
}: {
  i: number;
  mod: string;
  icon: ReactNode;
  title: string;
  href: string;
  /** Takes a whole row of the grid. */
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <article className={`dash-card${wide ? " is-wide" : ""}`} style={{ "--mod": `var(--m-${mod})`, "--i": i } as CSSProperties}>
      <header className="dash-card-head">
        <span className="dash-card-icon" aria-hidden="true">
          {icon}
        </span>
        <h2>{title}</h2>
        <Link href={href} className="dash-card-link">
          Abrir
        </Link>
      </header>
      <div className="dash-card-body">{children}</div>
    </article>
  );
}

function Unavailable() {
  return <p className="dash-muted">Sem resposta do servidor.</p>;
}

export function HomeDashboard({ data }: { data: DashData }) {
  const { finance, bills, training, diet, agenda, reminders, notes, documents, vault, boards, habits } = data;

  return (
    <div className="dash-grid">
      <Card i={0} mod="treino" icon={<IconDumbbell />} title="Treino de hoje" href="/treino">
        {!training ? (
          <Unavailable />
        ) : training.total === 0 ? (
          <p className="dash-muted">Nenhum treino cadastrado.</p>
        ) : training.today.length === 0 ? (
          <>
            <p className="dash-big">Descanso</p>
            {training.next && (
              <p className="dash-sub">
                Próximo: {training.next.name}, {training.next.day}
              </p>
            )}
          </>
        ) : (
          training.today.map((w) => (
            <div key={w.id} className="dash-block">
              <p className="dash-title">
                {w.name}
                {w.focus && <span>{w.focus}</span>}
              </p>
              <ul className="dash-rows">
                {w.exercises.slice(0, 6).map((e, k) => (
                  <li key={k}>
                    <span>{e.name}</span>
                    <b>{e.detail}</b>
                  </li>
                ))}
              </ul>
              {w.exercises.length > 6 && <p className="dash-sub">e mais {w.exercises.length - 6}</p>}
            </div>
          ))
        )}
      </Card>

      <Card i={1} mod="dieta" icon={<IconUtensils />} title="Dieta de hoje" href="/dieta">
        {!diet ? (
          <Unavailable />
        ) : diet.total === 0 ? (
          <p className="dash-muted">Nenhuma refeição cadastrada.</p>
        ) : diet.meals === 0 ? (
          <p className="dash-muted">Sem plano pra hoje.</p>
        ) : (
          <>
            <p className="dash-big">
              {fmt(diet.kcal)}
              <small> kcal</small>
              {diet.kcalTarget > 0 && <small className="dash-of"> de {fmt(diet.kcalTarget)}</small>}
            </p>
            <div className="dash-macros">
              {diet.macros.map((m) => (
                <div key={m.label} className="dash-macro">
                  <span>{m.label}</span>
                  <b>{fmt(m.value, 1)} g</b>
                  {m.target > 0 && (
                    <i aria-hidden="true">
                      <em style={{ width: `${Math.min(100, (m.value / m.target) * 100)}%` }} />
                    </i>
                  )}
                </div>
              ))}
            </div>
            <p className="dash-sub">
              {diet.now && `Agora: ${diet.now.name} (${diet.now.time})`}
              {diet.now && diet.next && ". "}
              {diet.next && `Próxima: ${diet.next.name} às ${diet.next.time}`}
            </p>
          </>
        )}
      </Card>

      <Card i={2} mod="habitos" icon={<IconHabit />} title="Hábitos de hoje" href="/habitos">
        {!habits ? (
          <Unavailable />
        ) : habits.total === 0 ? (
          <p className="dash-muted">Nenhum hábito cadastrado.</p>
        ) : habits.today.length === 0 ? (
          <p className="dash-muted">Nada previsto para hoje.</p>
        ) : (
          <>
            <p className="dash-big">
              {habits.today.filter((h) => h.done).length}
              <small> de {habits.today.length}</small>
            </p>
            <ul className="dash-habits">
              {habits.today.slice(0, 6).map((h) => (
                <li key={h.id} className={h.done ? "is-done" : ""} style={{ "--hb": h.color } as CSSProperties}>
                  <span className="dash-habit-mark" aria-label={h.done ? "feito" : "a fazer"} />
                  <span>{h.name}</span>
                  {h.progress && <b>{h.progress}</b>}
                </li>
              ))}
            </ul>
            {habits.today.length > 6 && <p className="dash-sub">e mais {habits.today.length - 6}</p>}
          </>
        )}
      </Card>

      <Card i={3} mod="financeiro" icon={<IconWallet />} title="Saldo do mês" href="/financeiro">
        {!finance ? (
          <Unavailable />
        ) : (
          <>
            <p className={`dash-big ${finance.saldoCents < 0 ? "is-bad" : "is-good"}`}>{finance.saldo}</p>
            <p className="dash-sub">{finance.month}</p>
            <div className="dash-pair">
              <div>
                <span>Entradas</span>
                <b>{finance.entradas}</b>
              </div>
              <div>
                <span>Saídas</span>
                <b>{finance.saidas}</b>
              </div>
            </div>
          </>
        )}
      </Card>

      <Card i={4} mod="financeiro" icon={<IconCard />} title="Contas" href="/financeiro/contas">
        {!bills ? (
          <Unavailable />
        ) : (
          <>
            <div className="dash-pair">
              <div>
                <span>A pagar</span>
                <b>{bills.payable}</b>
              </div>
              <div>
                <span>A receber</span>
                <b>{bills.receivable}</b>
              </div>
            </div>
            {bills.overdue > 0 && (
              <p className="dash-alert">{bills.overdue === 1 ? "1 conta vencida" : `${bills.overdue} contas vencidas`}</p>
            )}
            {bills.upcoming.length === 0 ? (
              <p className="dash-muted">Nenhuma conta em aberto.</p>
            ) : (
              <ul className="dash-rows">
                {bills.upcoming.map((b) => (
                  <li key={b.id} className={b.late ? "is-late" : ""}>
                    <span>
                      {b.description}
                      <small>{b.due}</small>
                    </span>
                    <b className={b.receivable ? "is-good" : ""}>{b.receivable ? `+${b.amount}` : b.amount}</b>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </Card>

      <Card i={5} mod="lembretes" icon={<IconBell />} title="Lembretes de hoje" href="/lembretes">
        {!reminders ? (
          <Unavailable />
        ) : reminders.items.length === 0 ? (
          <p className="dash-muted">{reminders.pending ? `Nada pra hoje. ${reminders.pending} pendentes sem data.` : "Nada pendente."}</p>
        ) : (
          <ul className="dash-rows">
            {reminders.items.map((r) => (
              <li key={r.id} className={r.late ? "is-late" : ""}>
                <span>{r.title}</span>
                <b>{r.when}</b>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card i={6} mod="notas" icon={<IconNote />} title="Últimas notas" href="/notas">
        {!notes ? (
          <Unavailable />
        ) : notes.length === 0 ? (
          <p className="dash-muted">Nenhuma nota ainda.</p>
        ) : (
          <ul className="dash-notes">
            {notes.map((n) => (
              <li key={n.id}>
                <b>{n.title}</b>
                {n.excerpt && <span>{n.excerpt}</span>}
                <small>{n.when}</small>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card i={7} mod="agenda" icon={<IconCalendar />} title="Próximos compromissos" href="/agenda">
        {!agenda ? (
          <Unavailable />
        ) : agenda.length === 0 ? (
          <p className="dash-muted">Nada nos próximos 14 dias.</p>
        ) : (
          <ul className="dash-rows">
            {agenda.map((e) => (
              <li key={e.id}>
                <span>
                  {e.title}
                  {e.location && <small>{e.location}</small>}
                </span>
                <b>{e.when}</b>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card i={8} mod="documentos" icon={<IconFolder />} title="Documentos" href="/documentos">
        {!documents ? (
          <Unavailable />
        ) : (
          <>
            <p className="dash-big">
              {fmt(documents.files)}
              <small> {documents.files === 1 ? "arquivo" : "arquivos"}</small>
            </p>
            <p className="dash-sub">
              {documents.folders === 0 ? "Nenhuma pasta ainda" : documents.folders === 1 ? "em 1 pasta" : `em ${documents.folders} pastas`}
            </p>
          </>
        )}
      </Card>

      <Card i={9} mod="senhas" icon={<IconLock />} title="Senhas" href="/senhas">
        {!vault ? (
          <p className="dash-muted">Cofre desativado neste servidor.</p>
        ) : (
          <>
            <p className="dash-big">
              {fmt(vault.count)}
              <small> {vault.count === 1 ? "senha" : "senhas"}</small>
            </p>
            <p className="dash-sub">Protegido por mTLS</p>
          </>
        )}
      </Card>

      <Card i={10} mod="quadros" icon={<IconFlow />} title="Quadros recentes" href="/quadros" wide>
        {!boards ? (
          <Unavailable />
        ) : boards.total === 0 ? (
          <p className="dash-muted">Nenhum quadro ainda.</p>
        ) : (
          <ul className="dash-boards">
            {boards.recent.map((b) => (
              <li key={b.id}>
                <Link href={`/quadros/${b.id}`} className="dash-board">
                  <span className="dash-board-thumb">
                    {b.preview && (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(b.preview)}`} alt="" />
                    )}
                  </span>
                  <b>{b.name}</b>
                  <small>{b.when}</small>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
