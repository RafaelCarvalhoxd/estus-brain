import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Estus Vault",
  description: "Controle financeiro pessoal do Estus Vault.",
};

// Applies the saved theme choice before the first paint — without this,
// a viewer who picked "Escuro" would see a flash of the light theme (or
// vice-versa) every time a page loads, because the CSS custom properties
// only respond to the data-theme attribute this sets.
const themeScript = `
  try {
    var t = localStorage.getItem("estus-theme");
    if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
  } catch (e) {}
`;

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>{children}</body>
    </html>
  );
}
