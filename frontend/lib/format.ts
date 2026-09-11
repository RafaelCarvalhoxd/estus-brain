export function deltaPercent(currentCents: number, previousCents: number): number | null {
  if (previousCents === 0) return null;
  return ((currentCents - previousCents) / previousCents) * 100;
}

export function formatPercent(value: number): string {
  return `${Math.abs(value).toLocaleString("pt-BR", { maximumFractionDigits: 1, minimumFractionDigits: value % 1 === 0 ? 0 : 1 })}%`;
}
