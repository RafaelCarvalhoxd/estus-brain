// Every module there is — what a screen needs to title itself and pick its
// color. The home screen decides layout and shortcut order on its own.
export type ModuleId = "financeiro" | "senhas" | "notas" | "lembretes" | "agenda" | "documentos" | "treino" | "dieta" | "quadros" | "habitos";

export interface ModuleDef {
  id: ModuleId;
  href: string;
  label: string;
  colorVar: string;
}

export const MODULE_META: Record<ModuleId, ModuleDef> = {
  financeiro: { id: "financeiro", href: "/financeiro", label: "Financeiro", colorVar: "--m-financeiro" },
  senhas: { id: "senhas", href: "/senhas", label: "Senhas", colorVar: "--m-senhas" },
  notas: { id: "notas", href: "/notas", label: "Notas", colorVar: "--m-notas" },
  lembretes: { id: "lembretes", href: "/lembretes", label: "Lembretes", colorVar: "--m-lembretes" },
  agenda: { id: "agenda", href: "/agenda", label: "Agenda", colorVar: "--m-agenda" },
  documentos: { id: "documentos", href: "/documentos", label: "Documentos", colorVar: "--m-documentos" },
  treino: { id: "treino", href: "/treino", label: "Treino", colorVar: "--m-treino" },
  dieta: { id: "dieta", href: "/dieta", label: "Dieta", colorVar: "--m-dieta" },
  quadros: { id: "quadros", href: "/quadros", label: "Quadros", colorVar: "--m-quadros" },
  habitos: { id: "habitos", href: "/habitos", label: "Hábitos", colorVar: "--m-habitos" },
};
