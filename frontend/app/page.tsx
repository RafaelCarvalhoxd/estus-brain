import { getMonthSummary } from "@/lib/api";
import { getBillSummary, getBillsReceived, listBills } from "@/lib/bills";
import { listReminders } from "@/lib/reminders";
import { listEvents } from "@/lib/agenda";
import { listNotes } from "@/lib/notes";
import { listVaultEntries } from "@/lib/vault";
import { countDocuments, listDocumentFolders } from "@/lib/documents";
import { listWorkouts } from "@/lib/training";
import { getDietTargets, listMeals } from "@/lib/diet";
import { listBoards } from "@/lib/boards";
import { listHabits } from "@/lib/habits";
import { sumMacros, formatAmount } from "@/lib/macros";
import { currentYearMonth, formatYearMonth } from "@/lib/month";
import { MODULE_META, type ModuleId } from "@/lib/modules";
import { DAY_LONG, TZ, dayKeyIn, minutesIn, plural, timeToMinutes, weekdayIn } from "@/lib/week";
import { BrainHub, type HubCard } from "@/components/brain/BrainHub";
import { BrainControls } from "@/components/brain/BrainControls";
import { ChatApp } from "@/components/assistant/ChatApp";
import { getConversation, listConversations } from "@/lib/assistant";
import { HomeDashboard, type DashData } from "@/components/HomeDashboard";
import { HomeView } from "@/components/HomeView";
import "./ui.css";
import "./home.css";
import "./chat/chat.css";

// Reading order: rows of left/right pairs, so the 1–9 and 0 shortcuts run the way
// the eye does. Top to bottom each side follows the brain — Quadros sits
// between the motor strip and the temporal lobe, like its region.
const LAYOUT: { id: ModuleId; side: "left" | "right" }[] = [
  { id: "financeiro", side: "left" },
  { id: "habitos", side: "right" },
  { id: "treino", side: "left" },
  { id: "agenda", side: "right" },
  { id: "quadros", side: "left" },
  { id: "dieta", side: "right" },
  { id: "notas", side: "left" },
  { id: "lembretes", side: "right" },
  { id: "documentos", side: "left" },
  { id: "senhas", side: "right" },
];

// Thumbnails are inlined into the home page, so a heavy one is left out.
const MAX_DASH_PREVIEW = 150_000;

function formatCents(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function formatSigned(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL", signDisplay: "exceptZero" });
}

function dayKey(d: Date): string {
  return d.toLocaleDateString("en-CA", { timeZone: TZ });
}

function timeLabel(d: Date): string {
  return d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit", timeZone: TZ });
}

function whenLabel(iso: string, now: Date): string {
  const d = new Date(iso);
  const tomorrow = new Date(now.getTime() + 864e5);
  if (dayKey(d) === dayKey(now)) return `hoje às ${timeLabel(d)}`;
  if (dayKey(d) === dayKey(tomorrow)) return `amanhã às ${timeLabel(d)}`;
  return d.toLocaleDateString("pt-BR", { day: "numeric", month: "short", timeZone: TZ }).replace(".", "");
}

// Bill due dates are plain calendar dates ("YYYY-MM-DD"); read them at noon
// so no time zone can push them onto the previous day.
function dueLabel(isoDate: string): string {
  const d = new Date(`${isoDate.slice(0, 10)}T12:00:00`);
  return d.toLocaleDateString("pt-BR", { day: "numeric", month: "short" }).replace(".", "");
}

const UNAVAILABLE = { text: "Sem resposta do servidor", alert: false };

export default async function HomePage({ searchParams }: { searchParams: Promise<{ c?: string }> }) {
  const { c: openConversation } = await searchParams;
  const month = currentYearMonth();
  const now = new Date();
  const in14Days = new Date(now.getTime() + 14 * 864e5);
  const today = weekdayIn(TZ, now);
  const nowMinutes = minutesIn(TZ, now);

  // Any single module being down (or disabled, like the vault without its
  // key) only blanks its own node and its own card.
  const [finance, received, billSummary, bills, vault, notes, reminders, events, docs, folders, workouts, meals, targets, boards, habits, conversations, conversation] =
    await Promise.allSettled([
      getMonthSummary(month),
      getBillsReceived(month),
      getBillSummary(),
      listBills(),
      listVaultEntries(),
      listNotes(),
      listReminders(),
      listEvents(now, in14Days),
      countDocuments(),
      listDocumentFolders(),
      listWorkouts(),
      listMeals(),
      getDietTargets(),
      listBoards(),
      listHabits(dayKeyIn(TZ, now), 1),
      listConversations(),
      openConversation ? getConversation(openConversation) : Promise.resolve(null),
    ]);

  const status: Record<ModuleId, { text: string; alert: boolean }> = {
    financeiro: UNAVAILABLE,
    senhas: { text: "Cofre trancado", alert: false },
    notas: UNAVAILABLE,
    lembretes: UNAVAILABLE,
    agenda: UNAVAILABLE,
    documentos: UNAVAILABLE,
    treino: UNAVAILABLE,
    dieta: UNAVAILABLE,
    quadros: UNAVAILABLE,
    habitos: UNAVAILABLE,
  };
  const dash: DashData = {
    finance: null,
    bills: null,
    training: null,
    diet: null,
    agenda: null,
    reminders: null,
    notes: null,
    documents: null,
    vault: null,
    boards: null,
    habits: null,
  };

  // ---- Financeiro ----
  if (finance.status === "fulfilled" && received.status === "fulfilled") {
    const entradas = received.value.cents;
    // Money that actually left: card purchases count once their invoice is paid.
    const saidas = finance.value.paid_out.cents;
    const saldo = entradas - saidas;
    status.financeiro = { text: `Saldo do mês ${formatSigned(saldo)}`, alert: saldo < 0 };
    dash.finance = {
      saldo: formatSigned(saldo),
      saldoCents: saldo,
      entradas: formatCents(entradas),
      saidas: formatCents(saidas),
      month: formatYearMonth(month),
    };
  }
  if (billSummary.status === "fulfilled" && bills.status === "fulfilled") {
    const open = bills.value
      .filter((b) => b.status === "pendente" || b.status === "atrasado")
      .sort((a, b) => a.due_date.localeCompare(b.due_date));
    dash.bills = {
      payable: billSummary.value.payable_open.formatted,
      receivable: billSummary.value.receivable_open.formatted,
      overdue: billSummary.value.overdue_count,
      upcoming: open.slice(0, 4).map((b) => ({
        id: b.id,
        description: b.description,
        amount: b.amount.formatted,
        due: b.status === "atrasado" ? `venceu ${dueLabel(b.due_date)}` : `vence ${dueLabel(b.due_date)}`,
        receivable: b.direction === "receber",
        late: b.status === "atrasado",
      })),
    };
  }

  // ---- Senhas ----
  if (vault.status === "fulfilled") {
    const n = vault.value.length;
    status.senhas = { text: n === 0 ? "Cofre vazio" : plural(n, "senha guardada", "senhas guardadas"), alert: false };
    dash.vault = { count: n };
  }

  // ---- Notas ----
  if (notes.status === "fulfilled") {
    const n = notes.value.length;
    status.notas = { text: n === 0 ? "Nenhuma nota ainda" : plural(n, "nota", "notas"), alert: false };
    dash.notes = [...notes.value]
      .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
      .slice(0, 3)
      .map((note) => {
        const body = note.body.replace(/\s+/g, " ").trim();
        return {
          id: note.id,
          title: note.title || body.slice(0, 40) || "Sem título",
          excerpt: note.title ? (body.length > 90 ? `${body.slice(0, 90)}…` : body) : "",
          when: whenLabel(note.updated_at, now),
        };
      });
  }

  // ---- Lembretes ----
  if (reminders.status === "fulfilled") {
    const open = reminders.value.filter((r) => !r.done);
    const late = open.filter((r) => r.due_at && new Date(r.due_at) < now);
    status.lembretes =
      open.length === 0
        ? { text: "Nada pendente", alert: false }
        : {
            text: late.length
              ? `${plural(open.length, "pendente", "pendentes")}, ${plural(late.length, "atrasado", "atrasados")}`
              : plural(open.length, "pendente", "pendentes"),
            alert: late.length > 0,
          };
    const dueToday = open.filter((r) => r.due_at && dayKey(new Date(r.due_at)) === dayKey(now) && new Date(r.due_at) >= now);
    const items = [...late, ...dueToday].sort((a, b) => (a.due_at ?? "").localeCompare(b.due_at ?? ""));
    dash.reminders = {
      pending: open.length,
      items: items.slice(0, 5).map((r) => {
        const d = new Date(r.due_at!);
        const isLate = d < now;
        const hasTime = timeLabel(d) !== "00:00";
        return {
          id: r.id,
          title: r.title,
          when: isLate ? (dayKey(d) === dayKey(now) ? "atrasado" : whenLabel(r.due_at!, now)) : hasTime ? timeLabel(d) : "hoje",
          late: isLate,
        };
      }),
    };
  }

  // ---- Agenda ----
  if (events.status === "fulfilled") {
    const upcoming = events.value
      .filter((e) => new Date(e.ends_at) >= now)
      .sort((a, b) => a.starts_at.localeCompare(b.starts_at));
    const next = upcoming[0];
    status.agenda = next
      ? { text: `${next.title}, ${whenLabel(next.starts_at, now)}`, alert: false }
      : { text: "Nada nos próximos 14 dias", alert: false };
    dash.agenda = upcoming.slice(0, 4).map((e) => ({
      id: e.id,
      title: e.title,
      when: whenLabel(e.starts_at, now),
      location: e.location,
    }));
  }

  // ---- Documentos ----
  if (docs.status === "fulfilled") {
    const n = docs.value.count;
    status.documentos = { text: n === 0 ? "Nenhum arquivo ainda" : plural(n, "arquivo", "arquivos"), alert: false };
    dash.documents = { files: n, folders: folders.status === "fulfilled" ? folders.value.length : 0 };
  }

  // ---- Hábitos ----
  if (habits.status === "fulfilled") {
    const dayKey = dayKeyIn(TZ, now);
    const list = habits.value.filter((h) => !h.archived);
    const todays = list.filter((h) => h.days.includes(today) && h.start_day <= dayKey);
    const done = todays.filter((h) => (h.logs[dayKey] ?? 0) >= h.target).length;
    status.habitos =
      list.length === 0
        ? { text: "Nenhum hábito ainda", alert: false }
        : todays.length === 0
          ? { text: "Nada previsto hoje", alert: false }
          : { text: done === todays.length ? "Tudo feito hoje" : `${done} de ${todays.length} hoje`, alert: false };
    dash.habits = {
      total: list.length,
      today: todays.map((h) => {
        const count = h.logs[dayKey] ?? 0;
        return {
          id: h.id,
          name: h.name,
          color: h.color,
          done: count >= h.target,
          progress: h.kind === "count" ? `${count}/${h.target}${h.unit ? ` ${h.unit}` : ""}` : "",
        };
      }),
    };
  }

  // ---- Quadros ----
  if (boards.status === "fulfilled") {
    const list = boards.value;
    status.quadros = { text: list.length === 0 ? "Nenhum quadro ainda" : plural(list.length, "quadro", "quadros"), alert: false };
    dash.boards = {
      total: list.length,
      recent: list.slice(0, 3).map((b) => ({
        id: b.id,
        name: b.name,
        preview: b.preview.length <= MAX_DASH_PREVIEW ? b.preview : "",
        when: whenLabel(b.updated_at, now),
      })),
    };
  }

  // ---- Treino ----
  if (workouts.status === "fulfilled") {
    const list = workouts.value;
    const todays = list.filter((w) => w.days.includes(today));
    status.treino =
      list.length === 0
        ? { text: "Nenhum treino cadastrado", alert: false }
        : todays.length
          ? { text: `Hoje: ${todays.map((w) => (w.focus ? `${w.name} (${w.focus})` : w.name)).join(", ")}`, alert: false }
          : { text: "Descanso hoje", alert: false };
    let next: { name: string; day: string } | null = null;
    if (list.length && !todays.length) {
      for (let i = 1; i <= 7; i++) {
        const d = (today + i) % 7;
        const w = list.find((x) => x.days.includes(d));
        if (w) {
          next = { name: w.name, day: DAY_LONG[d] };
          break;
        }
      }
    }
    dash.training = {
      total: list.length,
      next,
      today: todays.map((w) => ({
        id: w.id,
        name: w.name,
        focus: w.focus,
        exercises: w.exercises.map((e) => ({
          name: e.name,
          detail: [`${e.sets}×${e.reps || "?"}`, e.weight].filter(Boolean).join(", "),
        })),
      })),
    };
  }

  // ---- Dieta ----
  if (meals.status === "fulfilled") {
    const todays = meals.value.filter((m) => m.days.includes(today)).sort((a, b) => a.time.localeCompare(b.time));
    const started = todays.filter((m) => timeToMinutes(m.time) <= nowMinutes);
    const current = started[started.length - 1] ?? null;
    const next = todays.find((m) => timeToMinutes(m.time) > nowMinutes) ?? null;
    const totals = sumMacros(todays.flatMap((m) => m.items));
    status.dieta =
      meals.value.length === 0
        ? { text: "Nenhuma refeição cadastrada", alert: false }
        : todays.length === 0
          ? { text: "Sem plano pra hoje", alert: false }
          : next
            ? { text: `Próxima: ${next.name} às ${next.time}`, alert: false }
            : { text: `${formatAmount(totals.kcal)} kcal planejadas hoje`, alert: false };
    const t = targets.status === "fulfilled" ? targets.value : { kcal: 0, protein_g: 0, carbs_g: 0, fat_g: 0 };
    dash.diet = {
      total: meals.value.length,
      meals: todays.length,
      kcal: totals.kcal,
      kcalTarget: t.kcal,
      macros: [
        { label: "Proteína", value: totals.protein_g, target: t.protein_g },
        { label: "Carboidratos", value: totals.carbs_g, target: t.carbs_g },
        { label: "Gorduras", value: totals.fat_g, target: t.fat_g },
      ],
      now: current ? { name: current.name, time: current.time } : null,
      next: next ? { name: next.name, time: next.time } : null,
    };
  }

  const cards: HubCard[] = LAYOUT.map(({ id, side }) => ({
    ...MODULE_META[id],
    side,
    status: status[id].text,
    alert: status[id].alert,
  }));

  return (
    <HomeView hub={<BrainHub cards={cards} />} dashboard={<HomeDashboard data={dash} />} chat={
        <ChatApp
          initialConversations={conversations.status === "fulfilled" ? conversations.value : []}
          initialConversation={conversation.status === "fulfilled" ? conversation.value : null}
        />
      }
      controls={<BrainControls />}
      // A link to a conversation (?c=) opens the chat straight away.
      initialView={openConversation && conversation.status === "fulfilled" && conversation.value ? "chat" : "brain"}
    />
  );
}
