import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Estus Vault",
  description: "Controle financeiro pessoal do Estus Vault.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR">
      <body>{children}</body>
    </html>
  );
}
