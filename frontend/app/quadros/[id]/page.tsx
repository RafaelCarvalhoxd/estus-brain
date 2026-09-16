import { notFound } from "next/navigation";
import { getBoard } from "@/lib/boards";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { BoardEditor } from "@/components/boards/BoardEditor";
import { BoardTitle } from "@/components/boards/BoardTitle";
import "../../ui.css";
import "../boards.css";

export default async function QuadroPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const board = await getBoard(id);
  if (!board) notFound();

  return (
    <div className="shell board-shell">
      <ModuleTopBar module="quadros">
        <BoardTitle key={board.name} boardId={board.id} name={board.name} />
      </ModuleTopBar>
      <main className="main board-main">
        <BoardEditor boardId={board.id} name={board.name} scene={board.scene} />
      </main>
    </div>
  );
}
