import { getCardSpending, listCreditCards } from "@/lib/api";
import { CardSpendingDashboard } from "@/components/CardSpendingDashboard";
import { CreditCardManager } from "@/components/CreditCardManager";
import "./cards.css";

export default async function CartoesPage() {
  const [cards, overviews] = await Promise.all([listCreditCards(), getCardSpending()]);
  return (
    <>
      <CardSpendingDashboard overviews={overviews} />
      <CreditCardManager cards={cards} />
    </>
  );
}
