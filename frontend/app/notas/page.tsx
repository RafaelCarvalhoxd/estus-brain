import { listNotes } from "@/lib/notes";
import { Sidebar } from "@/components/Sidebar";
import { NotesBoard } from "@/components/NotesBoard";
import "../dashboard.css";
import "./notes.css";

export default async function NotasPage() {
  const notes = await listNotes();

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <div className="month-nav">
              <h1>Notas</h1>
            </div>
          </div>

          <NotesBoard notes={notes} />
        </div>
      </main>
    </div>
  );
}
