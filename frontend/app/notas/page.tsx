import { listNotes, listNoteCategories } from "@/lib/notes";
import { Sidebar } from "@/components/Sidebar";
import { NotesBoard } from "@/components/NotesBoard";
import "../ui.css";
import "./notes.css";

export default async function NotasPage() {
  const [notes, categories] = await Promise.all([listNotes(), listNoteCategories()]);

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Notas</h1>
          </div>

          <NotesBoard notes={notes} categories={categories} />
        </div>
      </main>
    </div>
  );
}
