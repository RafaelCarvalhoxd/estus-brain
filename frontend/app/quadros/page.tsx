import { listBoards } from "@/lib/boards";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { BoardsGallery } from "@/components/BoardsGallery";
import "../ui.css";
import "./boards.css";

export default async function QuadrosPage() {
  const boards = await listBoards();

  return (
    <div className="shell">
      <ModuleTopBar module="quadros" />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Quadros</h1>
          </div>
          <BoardsGallery boards={boards} />
        </div>
      </main>
    </div>
  );
}
