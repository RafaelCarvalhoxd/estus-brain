import { deltaPercent, formatPercent } from "@/lib/format";

export function DeltaPill({ currentCents, previousCents }: { currentCents: number; previousCents: number }) {
  const pct = deltaPercent(currentCents, previousCents);
  if (pct === null) {
    return <span className="pill">novo</span>;
  }
  if (Math.abs(pct) < 0.5) {
    return <span className="pill">— 0%</span>;
  }
  const rising = pct > 0;
  return (
    <span className={`pill ${rising ? "bad" : "good"}`}>
      {rising ? "▲" : "▼"} {formatPercent(pct)}
    </span>
  );
}
