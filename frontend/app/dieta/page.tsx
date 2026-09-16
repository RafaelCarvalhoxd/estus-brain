import type { CSSProperties } from "react";
import { getDietTargets, listMeals } from "@/lib/diet";
import { TZ, minutesIn, weekdayIn } from "@/lib/week";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { DietBoard } from "@/components/DietBoard";
import "../ui.css";
import "../plan.css";

export default async function DietaPage() {
  const [meals, targets] = await Promise.all([listMeals(), getDietTargets()]);
  // Day and clock come from the server in the owner's time zone, so "Agora"
  // and "Próxima" are right even if the server runs in UTC.
  const now = new Date();

  return (
    <div className="shell">
      <ModuleTopBar module="dieta" />
      <main className="main" style={{ "--mod": "var(--m-dieta)" } as CSSProperties}>
        <div className="wrap">
          <DietBoard meals={meals} targets={targets} today={weekdayIn(TZ, now)} nowMinutes={minutesIn(TZ, now)} />
        </div>
      </main>
    </div>
  );
}
