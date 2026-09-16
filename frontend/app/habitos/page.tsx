import type { CSSProperties } from "react";
import { listHabits } from "@/lib/habits";
import { dayKeyIn, TZ } from "@/lib/week";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { HabitsBoard } from "@/components/habits/HabitsBoard";
import "../ui.css";
import "../plan.css";
import "./habits.css";

// A year of history, week-aligned: 53 weeks back from today.
const HISTORY_DAYS = 371;

export default async function HabitosPage() {
  // "Today" is decided on the server in the owner's time zone, so the board
  // never flips days while hydrating.
  const today = dayKeyIn(TZ);
  const habits = await listHabits(today, HISTORY_DAYS);

  return (
    <div className="shell">
      <ModuleTopBar module="habitos" />
      <main className="main" style={{ "--mod": "var(--m-habitos)" } as CSSProperties}>
        <div className="wrap">
          <HabitsBoard habits={habits} today={today} historyDays={HISTORY_DAYS} />
        </div>
      </main>
    </div>
  );
}
