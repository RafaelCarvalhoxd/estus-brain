// Pasted and dropped images are stored inside the note itself, so they're
// scaled down first: a screenshot of a slide doesn't need to be 4K to be read.
const MAX_SIDE = 1600;

export function imageFiles(list: FileList | null | undefined): File[] {
  return Array.from(list ?? []).filter((f) => f.type.startsWith("image/"));
}

export async function imageToDataURL(file: File): Promise<string> {
  // An animated GIF would lose its animation on a canvas; keep small ones whole.
  if (file.type === "image/gif" && file.size < 1_500_000) return readAsDataURL(file);

  const bitmap = await createImageBitmap(file);
  const scale = Math.min(1, MAX_SIDE / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bitmap.width * scale);
  canvas.height = Math.round(bitmap.height * scale);
  canvas.getContext("2d")?.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  // WebP keeps transparency and text edges at a fraction of PNG's size.
  return canvas.toDataURL("image/webp", 0.88);
}

function readAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}
