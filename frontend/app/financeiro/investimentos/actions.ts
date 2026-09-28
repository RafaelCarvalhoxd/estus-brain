"use server";

import { revalidatePath } from "next/cache";
import {
  addContribution,
  createInvestment,
  deleteContribution,
  deleteInvestment,
  updateInvestment,
  type InvestmentKind,
  type RatePeriod,
} from "@/lib/investments";

export type InvestmentFields = {
  name: string;
  kind: InvestmentKind;
  rate: string;
  ratePeriod: RatePeriod;
};

type Result = { error?: string };

function parseAmountToCents(raw: string): number | null {
  const normalized = raw.replace(/[^\d,.-]/g, "").replace(/\./g, "").replace(",", ".");
  const value = Number.parseFloat(normalized);
  if (Number.isNaN(value) || value <= 0) return null;
  return Math.round(value * 100);
}

// "1", "1,5" or "0,85" percent → hundredths of a point. Empty means 0%.
function parseRate(raw: string): number | null {
  const trimmed = raw.replace("%", "").trim();
  if (trimmed === "") return 0;
  if (!/^\d+([.,]\d{1,2})?$/.test(trimmed)) return null;
  const bp = Math.round(Number.parseFloat(trimmed.replace(",", ".")) * 100);
  return bp <= 100_000 ? bp : null;
}

function isDate(raw: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(raw);
}

function readFields(f: InvestmentFields): { error: string } | { input: Parameters<typeof updateInvestment>[1] } {
  if (!f.name.trim()) return { error: "Nome é obrigatório." };
  const rate = parseRate(f.rate);
  if (rate === null) return { error: "Rendimento inválido. Use um número como 1 ou 0,85." };
  return { input: { name: f.name.trim(), kind: f.kind, rate_bp: rate, rate_period: f.ratePeriod } };
}

function message(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

// A contribution changes the month's outflow, so every screen showing the
// balance is stale after one.
function revalidateMoney() {
  revalidatePath("/");
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/investimentos");
}

export async function createInvestmentAction(fields: InvestmentFields, amount: string, date: string): Promise<Result> {
  const read = readFields(fields);
  if ("error" in read) return read;
  const cents = parseAmountToCents(amount);
  if (cents === null) return { error: "Valor inválido." };
  if (!isDate(date)) return { error: "Data inválida." };
  try {
    await createInvestment({ ...read.input, amount_cents: cents, date });
  } catch (err) {
    return { error: message(err, "Falha ao salvar.") };
  }
  revalidateMoney();
  return {};
}

export async function updateInvestmentAction(id: string, fields: InvestmentFields): Promise<Result> {
  const read = readFields(fields);
  if ("error" in read) return read;
  try {
    await updateInvestment(id, read.input);
  } catch (err) {
    return { error: message(err, "Falha ao salvar.") };
  }
  revalidateMoney();
  return {};
}

export async function deleteInvestmentAction(id: string): Promise<Result> {
  try {
    await deleteInvestment(id);
  } catch (err) {
    return { error: message(err, "Falha ao excluir.") };
  }
  revalidateMoney();
  return {};
}

export async function contributeAction(id: string, amount: string, date: string): Promise<Result> {
  const cents = parseAmountToCents(amount);
  if (cents === null) return { error: "Valor inválido." };
  if (!isDate(date)) return { error: "Data inválida." };
  try {
    await addContribution(id, cents, date);
  } catch (err) {
    return { error: message(err, "Falha ao salvar.") };
  }
  revalidateMoney();
  return {};
}

export async function deleteContributionAction(id: string, contributionId: string): Promise<Result> {
  try {
    await deleteContribution(id, contributionId);
  } catch (err) {
    if (err instanceof Error && /-> 409:/.test(err.message)) {
      return { error: "Este é o único aporte. Para devolver o valor, exclua o investimento." };
    }
    return { error: message(err, "Falha ao excluir.") };
  }
  revalidateMoney();
  return {};
}
