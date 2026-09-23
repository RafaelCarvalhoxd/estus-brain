import { LoginForm } from "./LoginForm";
import "../ui.css";

export default async function LoginPage({ searchParams }: PageProps<"/entrar">) {
  const { de } = await searchParams;
  return (
    <main style={{ minHeight: "100vh", display: "grid", placeItems: "center", padding: 16 }}>
      <div className="panel" style={{ width: "100%", maxWidth: 380 }}>
        <div className="panel-head">
          <h2>Estus Brain</h2>
        </div>
        <LoginForm from={typeof de === "string" ? de : ""} />
      </div>
    </main>
  );
}
