import type { Metadata } from "next";
import { Manrope } from "next/font/google";
import "./globals.css";

const manrope = Manrope({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Estus Brain",
  description: "Seu cérebro pessoal: finanças, agenda, treino, dieta, notas, lembretes, documentos e senhas.",
};

// Applies the saved theme choice before the first paint. Dark is the
// default (the bare :root palette), so only an explicit "light" needs the
// attribute — without this a viewer who picked "Claro" would see a flash
// of the dark theme every time a page loads.
const themeScript = `
  try {
    if (localStorage.getItem("estus-theme") === "light") document.documentElement.dataset.theme = "light";
  } catch (e) {}
`;

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR" className={manrope.variable} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>{children}</body>
    </html>
  );
}
