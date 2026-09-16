// Must be imported before @excalidraw/excalidraw: the editor resolves its
// font URLs against this path, and would otherwise fetch them from a public
// CDN. The files are copied into public/ by scripts/copy-excalidraw-assets.mjs.
declare global {
  interface Window {
    EXCALIDRAW_ASSET_PATH?: string | string[];
  }
}

if (typeof window !== "undefined") {
  window.EXCALIDRAW_ASSET_PATH = "/excalidraw-assets/";
}

export {};
