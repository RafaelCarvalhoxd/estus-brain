import { Sidebar } from "@/components/Sidebar";
import { VaultList } from "@/components/VaultList";
import { VaultEntryForm } from "@/components/VaultEntryForm";
import { VaultSetup } from "@/components/VaultSetup";
import { getVaultWebAuthnStatus, listVaultEntries } from "@/lib/vault";
import "../dashboard.css";
import "./vault.css";

export default async function SenhasPage() {
  const [entries, webauthnStatus] = await Promise.all([listVaultEntries(), getVaultWebAuthnStatus()]);

  return (
    <div className="shell">
      <Sidebar />
      <main className="main">
        <div className="wrap">
          <div className="topbar">
            <h1 className="vault-title">Senhas</h1>
          </div>

          {!webauthnStatus.registered && <VaultSetup />}

          <section className="bottom-split">
            <div className="panel">
              <div className="panel-head">
                <h2>Suas senhas</h2>
              </div>
              <VaultList entries={entries} canReveal={webauthnStatus.registered} />
            </div>

            <div className="panel">
              <div className="panel-head">
                <h2>Nova senha</h2>
              </div>
              <VaultEntryForm />
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}
