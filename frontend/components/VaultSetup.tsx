"use client";

import { useState } from "react";
import { creationOptionsFromServer, registrationCredentialToJSON } from "@/lib/webauthn-encoding";

type Status = "idle" | "working" | "error" | "done";

export function VaultSetup() {
  const [status, setStatus] = useState<Status>("idle");
  const [message, setMessage] = useState<string>("");

  async function configurar() {
    setStatus("working");
    setMessage("");
    try {
      const beginRes = await fetch("/api/vault-webauthn/register/begin", { method: "POST" });
      if (!beginRes.ok) throw new Error("Não foi possível iniciar o cadastro.");
      const begin = await beginRes.json();

      const credential = (await navigator.credentials.create(
        creationOptionsFromServer(begin.options.publicKey),
      )) as PublicKeyCredential | null;
      if (!credential) throw new Error("Nenhuma credencial foi criada.");

      const finishRes = await fetch(
        `/api/vault-webauthn/register/finish?session=${encodeURIComponent(begin.reveal_session)}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(registrationCredentialToJSON(credential)),
        },
      );
      if (!finishRes.ok) throw new Error("Não foi possível confirmar o cadastro.");

      setStatus("done");
    } catch (err) {
      setStatus("error");
      setMessage(friendlyMessage(err));
    }
  }

  return (
    <div className="panel vault-setup">
      <div className="panel-head">
        <h2>Configurar Touch ID</h2>
      </div>
      <p className="empty-note">
        Para revelar uma senha, o vault confirma sua identidade com Touch ID ou Face ID. Configure uma vez para
        habilitar isso neste dispositivo.
      </p>
      <button className="btn-primary" type="button" onClick={configurar} disabled={status === "working"}>
        {status === "working" ? "Aguardando confirmação…" : "Configurar Touch ID"}
      </button>
      {status === "error" && <p className="form-error">{message}</p>}
      {status === "done" && <p className="form-success">Touch ID configurado. Atualize a página para continuar.</p>}
    </div>
  );
}

function friendlyMessage(err: unknown): string {
  if (err instanceof DOMException && err.name === "NotAllowedError") {
    return "Cadastro cancelado.";
  }
  if (err instanceof Error) return err.message;
  return "Algo deu errado ao configurar o Touch ID.";
}
