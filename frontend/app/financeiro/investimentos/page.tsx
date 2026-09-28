import { listInvestments } from "@/lib/investments";
import { InvestmentsBoard } from "@/components/InvestmentsBoard";
import "./investments.css";

export default async function InvestimentosPage() {
  const data = await listInvestments();
  return <InvestmentsBoard data={data} />;
}
