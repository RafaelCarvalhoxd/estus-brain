import Link from "next/link";
import { listDocumentFolders, listDocuments, type DocumentFolder } from "@/lib/documents";
import { ModuleTopBar } from "@/components/ModuleTopBar";
import { DocumentsBrowser } from "@/components/DocumentsBrowser";
import "../ui.css";
import "./documents.css";

// Walks a folder back to the root so the header can show where you are.
function breadcrumbOf(folders: DocumentFolder[], folderId: string | null): DocumentFolder[] {
  const byId = new Map(folders.map((f) => [f.id, f]));
  const trail: DocumentFolder[] = [];
  let current = folderId ? byId.get(folderId) : undefined;
  while (current) {
    trail.unshift(current);
    current = current.parent_id ? byId.get(current.parent_id) : undefined;
  }
  return trail;
}

export default async function DocumentosPage({
  searchParams,
}: {
  searchParams: Promise<{ folder?: string }>;
}) {
  const params = await searchParams;
  const folderId = params.folder ?? null;

  const [folders, documents] = await Promise.all([listDocumentFolders(), listDocuments(folderId)]);
  const children = folders.filter((f) => (f.parent_id ?? null) === folderId);
  const trail = breadcrumbOf(folders, folderId);

  return (
    <div className="shell">
      <ModuleTopBar module="documentos" />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">{trail.length ? trail[trail.length - 1].name : "Documentos"}</h1>
          </div>

          <nav className="doc-trail" aria-label="Caminho">
            <Link href="/documentos" className={folderId ? "" : "is-current"}>
              Início
            </Link>
            {trail.map((f, i) => (
              <span key={f.id}>
                <span className="doc-trail-sep" aria-hidden="true">
                  /
                </span>
                <Link href={`/documentos?folder=${f.id}`} className={i === trail.length - 1 ? "is-current" : ""}>
                  {f.name}
                </Link>
              </span>
            ))}
          </nav>

          <DocumentsBrowser folderId={folderId} folders={children} documents={documents} />
        </div>
      </main>
    </div>
  );
}
