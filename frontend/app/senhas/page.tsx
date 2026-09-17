import { ModuleTopBar } from "@/components/ModuleTopBar";
import { VaultList } from "@/components/VaultList";
import { NewVaultEntryModal } from "@/components/NewVaultEntryModal";
import { listVaultEntries } from "@/lib/vault";
import "../ui.css";
import "./vault.css";

export default async function SenhasPage() {
  const entries = await listVaultEntries();

  return (
    <div className="shell">
      <ModuleTopBar module="senhas" />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Senhas</h1>
            <NewVaultEntryModal />
          </div>

          <div className="panel">
            <div className="panel-head">
              <h2>Suas senhas</h2>
            </div>
            <VaultList entries={entries} />
          </div>
        </div>
      </main>
    </div>
  );
}
