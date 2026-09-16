// Copies the Excalidraw editor's fonts into public/ so the whiteboard works
// without reaching a CDN — the app lives on a private server. Runs before
// dev and build; the copy is generated, so it's gitignored.
import { cpSync, existsSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const source = join(root, "node_modules/@excalidraw/excalidraw/dist/prod/fonts");
const target = join(root, "public/excalidraw-assets/fonts");

if (!existsSync(source)) {
  console.error(`excalidraw fonts not found at ${source} — run npm install`);
  process.exit(1);
}

rmSync(target, { recursive: true, force: true });
cpSync(source, target, { recursive: true });
