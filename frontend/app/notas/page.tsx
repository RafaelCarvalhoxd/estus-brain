import { getNote, listNoteCategories, listNotes } from "@/lib/notes";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { NotesApp } from "@/components/notes/NotesApp";
import "../ui.css";
import "./notes.css";

export default async function NotasPage({ searchParams }: { searchParams: Promise<{ n?: string; c?: string }> }) {
  const { n, c } = await searchParams;
  const [notes, categories, openNote] = await Promise.all([listNotes(), listNoteCategories(), n ? getNote(n) : null]);

  return (
    <div className="shell notes-shell">
      <ModuleTopBar module="notas" />
      <main className="main notes-main">
        <NotesApp notes={notes} categories={categories} openNote={openNote} filter={c ?? null} />
      </main>
    </div>
  );
}
