import { listCreditCards } from "@/lib/api";
import { CreditCardManager } from "@/components/CreditCardManager";

export default async function CartoesPage() {
  const cards = await listCreditCards();
  return <CreditCardManager cards={cards} />;
}
