/* eslint-disable @typescript-eslint/no-explicit-any -- tool results are the backend's loose JSON, read field by field */
import type { CardData, ModuleKey } from "./types";

// Ready-made flows: things people ask all the time, answered without any AI.
// Each one optionally asks for a few fields, runs one tool of the assistant
// (the same tools MCP and the AI engines use), and turns the result into a
// short sentence and a card.

export type OptionSource = "categories" | "cards" | "openBills" | "pendingReminders" | "upcomingEvents" | "todayHabits";

export interface FlowField {
  name: string;
  label: string;
  type: "text" | "textarea" | "number" | "money" | "date" | "datetime" | "month" | "select";
  required?: boolean;
  placeholder?: string;
  options?: { value: string; label: string }[];
  source?: OptionSource;
  initial?: () => string;
  /** Only shown when this returns true for the current values. */
  when?: (values: Record<string, string>) => boolean;
}

export interface Flow {
  id: string;
  module: ModuleKey;
  label: string;
  /** What appears as the person's message when the flow starts. */
  ask: string;
  /** The assistant's line above the form. */
  intro?: string;
  fields?: FlowField[];
  /** Fields repeated per row (several items at once), with `fields` shared. */
  rowFields?: FlowField[];
  popular?: boolean;
  keywords: string[];
  run: (values: Record<string, string>, rows: Record<string, string>[]) => { tool: string; args: Record<string, unknown> };
  present: (result: any, values: Record<string, string>) => { text: string; card?: CardData };
}

export const MODULES: { key: ModuleKey; label: string; color: string }[] = [
  { key: "geral", label: "Tudo", color: "var(--brain-glow)" },
  { key: "financeiro", label: "Financeiro", color: "var(--m-financeiro)" },
  { key: "contas", label: "Contas", color: "var(--m-financeiro)" },
  { key: "notas", label: "Notas", color: "var(--m-notas)" },
  { key: "lembretes", label: "Lembretes", color: "var(--m-lembretes)" },
  { key: "agenda", label: "Agenda", color: "var(--m-agenda)" },
  { key: "habitos", label: "Hábitos", color: "var(--m-habitos)" },
  { key: "treino", label: "Treino", color: "var(--m-treino)" },
  { key: "dieta", label: "Dieta", color: "var(--m-dieta)" },
  { key: "documentos", label: "Documentos", color: "var(--m-documentos)" },
];

const PAYMENT = [
  { value: "pix", label: "Pix" },
  { value: "debito", label: "Débito" },
  { value: "credito", label: "Crédito" },
];

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
const fmtDay = (d: string) => (d ? d.slice(0, 10).split("-").reverse().join("/") : "");
const fmtDateTime = (d: string) => (d ? `${fmtDay(d)} ${d.slice(11, 16)}` : "");
const monthLabel = (ym: string) => {
  const [y, m] = ym.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, 15)).toLocaleDateString("pt-BR", { month: "long", year: "numeric", timeZone: "UTC" });
};
const statusLabel: Record<string, string> = { atrasado: "Atrasada", pendente: "Pendente", pago: "Paga", recebido: "Recebida" };

function billsCard(r: any, title: string, empty: string): { text: string; card?: CardData } {
  const rows: any[] = r.contas ?? [];
  if (rows.length === 0) return { text: empty };
  const late = rows.filter((b) => b.status === "atrasado").length;
  const open = rows.filter((b) => b.status === "atrasado" || b.status === "pendente");
  const totalParts = [
    r.total_a_pagar !== "R$ 0,00" ? `${r.total_a_pagar} a pagar` : "",
    r.total_a_receber !== "R$ 0,00" ? `${r.total_a_receber} a receber` : "",
  ].filter(Boolean);
  return {
    text: `${plural(rows.length, "conta", "contas")}${totalParts.length ? `: ${totalParts.join(" e ")}` : ""}.${late ? ` ${plural(late, "está atrasada", "estão atrasadas")}.` : ""}`,
    card: {
      kind: "table",
      title,
      columns: ["Conta", "Valor", "Vencimento", "Status"],
      rows: rows.map((b) => [b.descricao, b.valor, fmtDay(b.vencimento), statusLabel[b.status] ?? b.status]),
      action: open.length === rows.length
        ? { label: "Marcar como paga", tool: "bills_mark_paid", args: rows.map((b) => ({ id: b.id })) }
        : undefined,
    },
  };
}

function remindersCard(r: any, empty: string): { text: string; card?: CardData } {
  const items: any[] = r.lembretes ?? [];
  if (items.length === 0) return { text: empty };
  return {
    text: items.length ? plural(items.length, "lembrete", "lembretes") + "." : empty,
    card: {
      kind: "table",
      columns: ["Lembrete", "Quando"],
      rows: items.map((x) => [x.titulo + (x.atrasado ? " (atrasado)" : ""), x.quando ? fmtDateTime(x.quando) : "Sem data"]),
      action: items.some((x) => !x.feito)
        ? { label: "Concluir", tool: "reminders_set_done", args: items.map((x) => ({ id: x.id, done: true })) }
        : undefined,
    },
  };
}

export const FLOWS: Flow[] = [
  // ---- Tudo
  {
    id: "overview",
    module: "geral",
    label: "Como está meu dia?",
    ask: "Como está meu dia?",
    popular: true,
    keywords: ["dia", "hoje", "resumo", "panorama", "como esta"],
    run: () => ({ tool: "overview_today", args: {} }),
    present: (r) => {
      const items: { label: string; value: string; tone?: "good" | "bad" | "muted" }[] = [];
      if (r.gastos_do_mes?.total) items.push({ label: "Gasto no mês", value: r.gastos_do_mes.total });
      if (r.contas?.a_pagar) items.push({ label: "Contas em aberto", value: r.contas.a_pagar, tone: r.contas.atrasadas ? "bad" : undefined });
      if (r.contas?.atrasadas !== undefined) items.push({ label: "Atrasadas", value: String(r.contas.atrasadas), tone: r.contas.atrasadas ? "bad" : "muted" });
      if (r.lembretes?.lembretes) items.push({ label: "Lembretes pendentes", value: String(r.lembretes.lembretes.length) });
      if (r.habitos?.total !== undefined) items.push({ label: "Hábitos de hoje", value: `${r.habitos.feitos} de ${r.habitos.total}` });
      if (r.compromissos?.compromissos) items.push({ label: "Compromissos (2 dias)", value: String(r.compromissos.compromissos.length) });
      if (r.treino) items.push({ label: "Treino", value: r.treino.descanso ? "Descanso" : (r.treino.treinos ?? []).map((t: any) => t.treino).join(", ") });
      const events = (r.compromissos?.compromissos ?? []).slice(0, 3).map((e: any) => `${e.titulo} em ${fmtDateTime(e.inicio)}`);
      return {
        text: `Hoje é ${fmtDay(r.hoje)}.${events.length ? ` Próximos compromissos: ${events.join("; ")}.` : ""}`,
        card: { kind: "stats", items },
      };
    },
  },

  // ---- Financeiro
  {
    id: "txn-create",
    module: "financeiro",
    label: "Lançar um gasto",
    ask: "Quero lançar um gasto",
    intro: "Beleza. Preencha os dados do gasto:",
    popular: true,
    keywords: ["lancar", "gasto", "gastei", "comprei", "paguei", "despesa", "lancamento"],
    fields: [
      { name: "description", label: "Descrição", type: "text", required: true, placeholder: "Mercado" },
      { name: "amount", label: "Valor (R$)", type: "money", required: true, placeholder: "0,00" },
      { name: "category", label: "Categoria", type: "select", required: true, source: "categories" },
      { name: "payment_method", label: "Pagamento", type: "select", options: PAYMENT, initial: () => "pix" },
      { name: "credit_card", label: "Cartão", type: "select", source: "cards", when: (v) => v.payment_method === "credito" },
      { name: "installments", label: "Parcelas", type: "number", initial: () => "1", when: (v) => v.payment_method === "credito" },
      { name: "date", label: "Data", type: "date", initial: () => todayISO() },
    ],
    run: (v) => ({
      tool: "finance_create_transaction",
      args: { ...v, amount: parseMoney(v.amount), installments: Number(v.installments || 1) },
    }),
    present: (r) => ({
      text: `Lancei ${r.descricao} de ${r.valor} em ${r.categoria}.`,
      card: {
        kind: "done",
        title: "Gasto lançado",
        lines: [
          `${r.descricao}: ${r.valor}`,
          `${r.categoria}, ${r.pagamento}${r.parcelas > 1 ? ` em ${r.parcelas}x` : ""}`,
          `Conta no mês de ${monthLabel(r.mes_competencia)}`,
        ],
      },
    }),
  },
  {
    id: "txn-create-many",
    module: "financeiro",
    label: "Lançar vários gastos",
    ask: "Quero lançar vários gastos de uma vez",
    intro: "Adicione uma linha por gasto:",
    keywords: ["varios", "lancamentos", "gastos", "lote", "de uma vez"],
    fields: [{ name: "date", label: "Data de todos", type: "date", initial: () => todayISO() }],
    rowFields: [
      { name: "description", label: "Descrição", type: "text", required: true },
      { name: "amount", label: "Valor", type: "money", required: true },
      { name: "category", label: "Categoria", type: "select", required: true, source: "categories" },
      { name: "payment_method", label: "Pagamento", type: "select", options: PAYMENT.filter((p) => p.value !== "credito"), initial: () => "pix" },
    ],
    run: (v, rows) => ({
      tool: "finance_create_transactions",
      args: { items: rows.map((row) => ({ ...row, amount: parseMoney(row.amount), date: v.date })) },
    }),
    present: (r) => ({
      text: `Lancei ${plural((r.lancados ?? []).length, "gasto", "gastos")}, total de ${r.total}.${r.falhas?.length ? ` ${plural(r.falhas.length, "falhou", "falharam")}.` : ""}`,
      card: {
        kind: "table",
        title: "Lançados",
        columns: ["Descrição", "Valor", "Categoria"],
        rows: (r.lancados ?? []).map((x: any) => [x.descricao, x.valor, x.categoria]),
        footer: r.falhas?.length ? r.falhas.join(" · ") : `Total: ${r.total}`,
      },
    }),
  },
  {
    id: "month-summary",
    module: "financeiro",
    label: "Quanto gastei este mês?",
    ask: "Quanto gastei este mês?",
    popular: true,
    keywords: ["quanto gastei", "gastos do mes", "total", "resumo financeiro", "gastei este mes"],
    run: () => ({ tool: "finance_month_summary", args: {} }),
    present: (r) => ({
      text: `Em ${monthLabel(r.mes)} você gastou ${r.total}, contra ${r.mes_anterior} no mês anterior.`,
      card: {
        kind: "table",
        title: "Por categoria",
        columns: ["Categoria", "Gasto", "Orçamento"],
        rows: (r.por_categoria ?? []).map((c: any) => [c.categoria, c.total, c.orcamento ? `${c.orcamento}${c.estourou ? " (estourou)" : ""}` : "—"]),
        footer: `${plural(r.num_lancamentos, "lançamento", "lançamentos")} · fixos ${r.fixos}`,
      },
    }),
  },
  {
    id: "month-summary-pick",
    module: "financeiro",
    label: "Gastos de outro mês",
    ask: "Quero ver os gastos de um mês",
    intro: "De qual mês?",
    keywords: ["mes passado", "outro mes", "comparar"],
    fields: [{ name: "month", label: "Mês", type: "month", required: true, initial: () => todayISO().slice(0, 7) }],
    run: (v) => ({ tool: "finance_month_summary", args: { month: v.month } }),
    present: (r) => FLOWS.find((f) => f.id === "month-summary")!.present(r, {}),
  },
  {
    id: "txn-list",
    module: "financeiro",
    label: "Ver lançamentos",
    ask: "Quero ver meus lançamentos",
    intro: "Filtre se quiser:",
    keywords: ["lancamentos", "extrato", "listar gastos", "excluir lancamento", "apagar gasto"],
    fields: [
      { name: "month", label: "Mês", type: "month", initial: () => todayISO().slice(0, 7) },
      { name: "category", label: "Categoria", type: "select", source: "categories" },
      { name: "search", label: "Texto", type: "text", placeholder: "Mercado" },
    ],
    run: (v) => ({ tool: "finance_list_transactions", args: { ...v, limit: 50 } }),
    present: (r) => {
      const rows: any[] = r.lancamentos ?? [];
      return {
        text: rows.length ? `${plural(rows.length, "lançamento", "lançamentos")} em ${monthLabel(r.mes)}, somando ${r.total_filtrado}.` : "Nenhum lançamento com esse filtro.",
        card: {
          kind: "table",
          columns: ["Data", "Descrição", "Categoria", "Valor"],
          rows: rows.map((t) => [fmtDay(t.data), t.descricao + (t.parcela ? ` (${t.parcela})` : ""), t.categoria, t.valor]),
          action: rows.length
            ? { label: "Excluir", tool: "finance_delete_transaction", confirm: "Excluir este lançamento?", args: rows.map((t) => ({ id: t.id, confirm: true })) }
            : undefined,
        },
      };
    },
  },

  // ---- Contas
  {
    id: "bills-next-month",
    module: "contas",
    label: "Contas a pagar do mês que vem",
    ask: "Quais minhas contas a pagar no mês que vem?",
    popular: true,
    keywords: ["contas a pagar", "mes que vem", "proximo mes", "vencer", "boletos"],
    run: () => ({ tool: "bills_list", args: { direction: "pagar", month: "proximo" } }),
    present: (r) => billsCard(r, `A pagar em ${r.mes ? monthLabel(r.mes) : "breve"}`, "Nenhuma conta a pagar no mês que vem."),
  },
  {
    id: "bills-this-month",
    module: "contas",
    label: "Contas deste mês",
    ask: "Quais contas vencem este mês?",
    keywords: ["contas do mes", "este mes", "vencem"],
    run: () => ({ tool: "bills_list", args: { month: "atual" } }),
    present: (r) => billsCard(r, "Em aberto este mês", "Nenhuma conta em aberto este mês."),
  },
  {
    id: "bills-late",
    module: "contas",
    label: "Contas atrasadas",
    ask: "Tenho contas atrasadas?",
    popular: true,
    keywords: ["atrasadas", "atrasada", "vencidas", "vencida"],
    run: () => ({ tool: "bills_list", args: { status: "atrasadas" } }),
    present: (r) => billsCard(r, "Atrasadas", "Nenhuma conta atrasada."),
  },
  {
    id: "bills-receive",
    module: "contas",
    label: "A receber",
    ask: "O que tenho a receber?",
    keywords: ["receber", "entrada", "vou receber"],
    run: () => ({ tool: "bills_list", args: { direction: "receber" } }),
    present: (r) => billsCard(r, "A receber", "Nada a receber em aberto."),
  },
  {
    id: "bill-create",
    module: "contas",
    label: "Cadastrar conta",
    ask: "Quero cadastrar uma conta",
    intro: "Preencha a conta:",
    keywords: ["nova conta", "cadastrar conta", "boleto", "adicionar conta"],
    fields: [
      { name: "description", label: "Descrição", type: "text", required: true, placeholder: "Aluguel" },
      { name: "amount", label: "Valor (R$)", type: "money", required: true },
      { name: "due_date", label: "Vencimento", type: "date", required: true, initial: () => todayISO() },
      { name: "direction", label: "Tipo", type: "select", options: [{ value: "pagar", label: "A pagar" }, { value: "receber", label: "A receber" }], initial: () => "pagar" },
      { name: "recurring", label: "Repete todo mês", type: "select", options: [{ value: "", label: "Não" }, { value: "true", label: "Sim" }] },
    ],
    run: (v) => ({ tool: "bills_create", args: { ...v, amount: parseMoney(v.amount), recurring: v.recurring === "true" } }),
    present: (r) => ({
      text: `Cadastrei ${r.descricao} de ${r.valor}, vencendo em ${fmtDay(r.vencimento)}.`,
      card: { kind: "done", title: r.tipo === "receber" ? "Conta a receber" : "Conta a pagar", lines: [`${r.descricao}: ${r.valor}`, `Vence em ${fmtDay(r.vencimento)}`] },
    }),
  },
  {
    id: "bill-pay",
    module: "contas",
    label: "Marcar conta como paga",
    ask: "Quero marcar uma conta como paga",
    intro: "Qual conta?",
    keywords: ["paguei a conta", "marcar como paga", "quitar", "pagar conta"],
    fields: [
      { name: "id", label: "Conta", type: "select", required: true, source: "openBills" },
      { name: "paid_on", label: "Pago em", type: "date", initial: () => todayISO() },
    ],
    run: (v) => ({ tool: "bills_mark_paid", args: v }),
    present: (r) => ({ text: `${r.descricao} marcada como ${r.tipo === "receber" ? "recebida" : "paga"}.`, card: { kind: "done", title: "Conta quitada", lines: [`${r.descricao}: ${r.valor}`] } }),
  },

  // ---- Notas
  {
    id: "note-create",
    module: "notas",
    label: "Anotar algo",
    ask: "Quero anotar uma coisa",
    intro: "Escreva a nota:",
    popular: true,
    keywords: ["anotar", "nota", "anota", "escrever", "lembrar disso"],
    fields: [
      { name: "title", label: "Título", type: "text", required: true },
      { name: "text", label: "Texto", type: "textarea" },
      { name: "notebook", label: "Caderno", type: "text", placeholder: "Geral" },
    ],
    run: (v) => ({ tool: "notes_create", args: v }),
    present: (r) => ({ text: `Anotado em ${r.caderno}.`, card: { kind: "done", title: r.titulo, lines: [`Caderno: ${r.caderno}`] } }),
  },
  {
    id: "notes-search",
    module: "notas",
    label: "Buscar nas notas",
    ask: "Quero procurar nas minhas notas",
    intro: "O que você procura?",
    keywords: ["buscar nota", "procurar", "onde anotei", "achar nota"],
    fields: [{ name: "query", label: "Texto", type: "text", required: true }],
    run: (v) => ({ tool: "notes_search", args: { query: v.query } }),
    present: (r) => {
      const notes: any[] = r.notas ?? [];
      return {
        text: notes.length ? `Encontrei ${plural(notes.length, "nota", "notas")}.` : "Nenhuma nota com esse texto.",
        card: { kind: "list", items: notes.map((n) => ({ title: n.titulo || "Sem título", subtitle: n.trecho, meta: n.caderno })), empty: "Nada encontrado." },
      };
    },
  },
  {
    id: "notes-recent",
    module: "notas",
    label: "Últimas notas",
    ask: "Quais foram minhas últimas notas?",
    keywords: ["ultimas notas", "notas recentes"],
    run: () => ({ tool: "notes_search", args: { limit: 8 } }),
    present: (r) => FLOWS.find((f) => f.id === "notes-search")!.present(r, {}),
  },

  // ---- Lembretes
  {
    id: "reminder-create",
    module: "lembretes",
    label: "Criar lembrete",
    ask: "Me lembra de uma coisa",
    intro: "Do que e quando?",
    popular: true,
    keywords: ["lembrete", "me lembra", "lembrar", "avisar"],
    fields: [
      { name: "title", label: "Lembrar de", type: "text", required: true },
      { name: "when", label: "Quando", type: "datetime" },
    ],
    run: (v) => ({ tool: "reminders_create", args: v }),
    present: (r) => ({ text: `Vou te lembrar${r.quando ? ` em ${fmtDateTime(r.quando)}` : ""}.`, card: { kind: "done", title: r.titulo, lines: [r.quando ? fmtDateTime(r.quando) : "Sem data"] } }),
  },
  {
    id: "reminders-today",
    module: "lembretes",
    label: "Lembretes de hoje",
    ask: "O que tenho para lembrar hoje?",
    keywords: ["lembretes de hoje", "hoje lembrar", "pendencias"],
    run: () => ({ tool: "reminders_list", args: { filter: "hoje" } }),
    present: (r) => remindersCard(r, "Nenhum lembrete para hoje."),
  },
  {
    id: "reminders-pending",
    module: "lembretes",
    label: "Todos os pendentes",
    ask: "Quais lembretes estão pendentes?",
    keywords: ["lembretes pendentes", "todos lembretes"],
    run: () => ({ tool: "reminders_list", args: { filter: "pendentes" } }),
    present: (r) => remindersCard(r, "Nenhum lembrete pendente."),
  },

  // ---- Agenda
  {
    id: "agenda-week",
    module: "agenda",
    label: "Próximos compromissos",
    ask: "Quais são meus próximos compromissos?",
    popular: true,
    keywords: ["compromissos", "agenda", "reuniao", "semana", "consulta"],
    run: () => ({ tool: "agenda_list", args: { days: 7 } }),
    present: (r) => {
      const events: any[] = r.compromissos ?? [];
      return {
        text: events.length ? `${plural(events.length, "compromisso", "compromissos")} nos próximos 7 dias.` : "Nada marcado nos próximos 7 dias.",
        card: {
          kind: "table",
          columns: ["Quando", "Compromisso", "Local"],
          rows: events.map((e) => [fmtDateTime(e.inicio), e.titulo, e.local || "—"]),
          action: events.length ? { label: "Excluir", tool: "agenda_delete", confirm: "Excluir este compromisso?", args: events.map((e) => ({ id: e.id, confirm: true })) } : undefined,
        },
      };
    },
  },
  {
    id: "event-create",
    module: "agenda",
    label: "Marcar compromisso",
    ask: "Quero marcar um compromisso",
    intro: "Preencha o compromisso:",
    keywords: ["marcar", "agendar", "novo compromisso"],
    fields: [
      { name: "title", label: "O quê", type: "text", required: true },
      { name: "start", label: "Quando", type: "datetime", required: true },
      { name: "duration_minutes", label: "Duração (min)", type: "number", initial: () => "60" },
      { name: "location", label: "Local", type: "text" },
    ],
    run: (v) => ({ tool: "agenda_create", args: { ...v, duration_minutes: Number(v.duration_minutes || 60) } }),
    present: (r) => ({ text: `Marquei ${r.titulo} em ${fmtDateTime(r.inicio)}.`, card: { kind: "done", title: r.titulo, lines: [`${fmtDateTime(r.inicio)} até ${r.fim.slice(11, 16)}`, r.local].filter(Boolean) } }),
  },

  // ---- Hábitos
  {
    id: "habits-today",
    module: "habitos",
    label: "Hábitos de hoje",
    ask: "Como estão meus hábitos hoje?",
    popular: true,
    keywords: ["habitos", "habito", "sequencia", "streak"],
    run: () => ({ tool: "habits_today", args: {} }),
    present: (r) => ({
      text: r.total ? `${r.feitos} de ${r.total} feitos hoje.` : "Nenhum hábito previsto para hoje.",
      card: {
        kind: "list",
        items: (r.habitos ?? []).map((h: any) => ({ title: h.habito, subtitle: h.progresso || (h.feito ? "Feito" : "A fazer"), meta: `sequência ${h.sequencia}`, done: h.feito })),
        empty: "Nada previsto hoje.",
      },
    }),
  },
  {
    id: "habit-log",
    module: "habitos",
    label: "Marcar hábito",
    ask: "Quero marcar um hábito",
    intro: "Qual hábito?",
    keywords: ["marcar habito", "fiz", "bebi", "li", "treinei"],
    fields: [
      { name: "habit", label: "Hábito", type: "select", required: true, source: "todayHabits" },
      { name: "add", label: "Quantidade a somar (só para contar)", type: "number", placeholder: "vazio = marcar como feito" },
    ],
    run: (v) => ({ tool: "habits_log", args: v.add ? { habit: v.habit, add: Number(v.add) } : { habit: v.habit, done: true } }),
    present: (r) => ({ text: r.feito ? `${r.habito}: feito!` : `${r.habito}: ${r.valor} de ${r.meta}.`, card: { kind: "done", title: r.habito, lines: [r.feito ? "Meta do dia batida" : `${r.valor} de ${r.meta}`] } }),
  },

  // ---- Treino e dieta
  {
    id: "training-today",
    module: "treino",
    label: "Treino de hoje",
    ask: "Qual é o meu treino de hoje?",
    popular: true,
    keywords: ["treino", "academia", "exercicios", "malhar"],
    run: () => ({ tool: "training_day", args: {} }),
    present: (r) => {
      const workouts: any[] = r.treinos ?? [];
      if (!workouts.length) return { text: "Hoje é dia de descanso.", card: { kind: "done", title: "Descanso", lines: ["Nenhum treino previsto para hoje."] } };
      const w = workouts[0];
      return {
        text: `Hoje é ${w.treino}${w.foco ? ` (${w.foco})` : ""}.`,
        card: { kind: "table", title: w.treino, columns: ["Exercício", "Séries", "Carga"], rows: (w.exercicios ?? []).map((e: any) => [e.exercicio, e.series, e.carga || "—"]) },
      };
    },
  },
  {
    id: "training-week",
    module: "treino",
    label: "Plano da semana",
    ask: "Qual é o meu plano de treino da semana?",
    keywords: ["plano de treino", "treinos da semana"],
    run: () => ({ tool: "training_day", args: { week: true } }),
    present: (r) => {
      const days = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];
      const workouts: any[] = r.treinos ?? [];
      return {
        text: workouts.length ? `${plural(workouts.length, "treino", "treinos")} no plano.` : "Nenhum treino cadastrado.",
        card: { kind: "list", items: workouts.map((w) => ({ title: w.treino, subtitle: w.foco, meta: (w.dias ?? []).map((d: number) => days[d]).join(", ") })), empty: "Nenhum treino." },
      };
    },
  },
  {
    id: "diet-today",
    module: "dieta",
    label: "Dieta de hoje",
    ask: "Como está minha dieta hoje?",
    popular: true,
    keywords: ["dieta", "refeicoes", "macros", "calorias", "proteina"],
    run: () => ({ tool: "diet_day", args: {} }),
    present: (r) => {
      const meals: any[] = r.refeicoes ?? [];
      const t = r.totais ?? {};
      const m = r.metas ?? {};
      return {
        text: meals.length ? `${plural(meals.length, "refeição", "refeições")} hoje, ${Math.round(t.kcal)} kcal${m.kcal ? ` de ${Math.round(m.kcal)}` : ""}.` : "Nenhuma refeição prevista hoje.",
        card: {
          kind: "stats",
          items: [
            { label: "Calorias", value: `${Math.round(t.kcal ?? 0)}${m.kcal ? ` / ${Math.round(m.kcal)}` : ""} kcal` },
            { label: "Proteína", value: `${t.proteina_g ?? 0}${m.proteina_g ? ` / ${m.proteina_g}` : ""} g` },
            { label: "Carboidratos", value: `${t.carboidratos_g ?? 0}${m.carboidratos_g ? ` / ${m.carboidratos_g}` : ""} g` },
            { label: "Gorduras", value: `${t.gordura_g ?? 0}${m.gordura_g ? ` / ${m.gordura_g}` : ""} g` },
          ],
        },
      };
    },
  },

  // ---- Documentos
  {
    id: "docs-search",
    module: "documentos",
    label: "Achar um documento",
    ask: "Quero achar um documento",
    intro: "Qual o nome (ou parte dele)?",
    popular: true,
    keywords: ["documento", "arquivo", "pdf", "achar arquivo"],
    fields: [{ name: "query", label: "Nome", type: "text", required: true }],
    run: (v) => ({ tool: "documents_search", args: v }),
    present: (r) => {
      const docs: any[] = r.documentos ?? [];
      return {
        text: docs.length ? `Encontrei ${plural(docs.length, "arquivo", "arquivos")}.` : "Nenhum arquivo com esse nome.",
        card: { kind: "list", items: docs.map((d) => ({ title: d.arquivo, subtitle: d.pasta, meta: fmtDay(d.enviado) })), empty: "Nada encontrado." },
      };
    },
  },
];

export function flowById(id: string): Flow | undefined {
  return FLOWS.find((f) => f.id === id);
}

export function todayISO(): string {
  return new Date().toLocaleDateString("en-CA", { timeZone: "America/Sao_Paulo" });
}

export function parseMoney(v: string): number {
  const clean = v.replace(/[^\d,.-]/g, "");
  // "1.234,56" and "1234,56" are Brazilian; "1234.56" is not.
  const normalized = clean.includes(",") ? clean.replace(/\./g, "").replace(",", ".") : clean;
  return Number(normalized);
}

export function normalize(s: string): string {
  return s.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
}

// Without an AI engine, free text is matched to flows by their keywords.
export function matchFlows(text: string, module: ModuleKey): { flow: Flow; score: number }[] {
  const q = normalize(text);
  return FLOWS.map((flow) => {
    let score = 0;
    for (const k of flow.keywords) if (q.includes(k)) score += k.includes(" ") ? 2 : 1;
    if (score && (module === "geral" || flow.module === module)) score += 0.5;
    return { flow, score };
  })
    .filter((m) => m.score > 0)
    .sort((a, b) => b.score - a.score);
}

// "gastei 45,90 no mercado" → prefills for the expense form.
export function expensePrefill(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  const amount = /(\d+(?:[.,]\d{1,2})?)/.exec(text);
  if (amount) out.amount = amount[1];
  const where = /\b(?:no|na|em|com|de)\s+([^\d,.;]+?)\s*$/i.exec(text.trim());
  if (where) out.description = where[1].trim().replace(/^\w/, (c) => c.toUpperCase());
  return out;
}
