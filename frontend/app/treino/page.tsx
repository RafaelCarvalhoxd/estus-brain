import type { CSSProperties } from "react";
import { listWorkouts } from "@/lib/training";
import { TZ, weekdayIn } from "@/lib/week";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { TrainingBoard } from "@/components/TrainingBoard";
import "../ui.css";
import "../plan.css";

export default async function TreinoPage() {
  const workouts = await listWorkouts();
  // "Today" is decided here, on the server, in the owner's time zone — the
  // board just renders it, so the page never flips days during hydration.
  const today = weekdayIn(TZ);

  return (
    <div className="shell">
      <ModuleTopBar module="treino" />
      <main className="main" style={{ "--mod": "var(--m-treino)" } as CSSProperties}>
        <div className="wrap">
          <TrainingBoard workouts={workouts} today={today} />
        </div>
      </main>
    </div>
  );
}
