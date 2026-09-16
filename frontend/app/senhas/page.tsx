import { ModuleTopBar } from "@/components/ModuleTopBar";
import { VaultList } from "@/components/VaultList";
import { VaultSetup } from "@/components/VaultSetup";
import { NewVaultEntryModal } from "@/components/NewVaultEntryModal";
import { getVaultWebAuthnStatus, listVaultEntries } from "@/lib/vault";
import "../ui.css";
import "./vault.css";

export default async function SenhasPage() {
  const [entries, webauthnStatus] = await Promise.all([listVaultEntries(), getVaultWebAuthnStatus()]);

  return (
    <div className="shell">
      <ModuleTopBar module="senhas" />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="page-title">Senhas</h1>
            <NewVaultEntryModal />
          </div>

          {!webauthnStatus.registered && <VaultSetup />}

          <div className="panel">
            <div className="panel-head">
              <h2>Suas senhas</h2>
            </div>
            <VaultList entries={entries} canReveal={webauthnStatus.registered} />
          </div>
        </div>
      </main>
    </div>
  );
}
