const MAX_EDGE = 1200;
const TARGET_BYTES = 200 * 1024;

// Compress BEFORE anything touches IndexedDB.
export async function compressImage(file: File): Promise<Blob> {
  // Decode honouring EXIF rotation so portrait shots don't arrive sideways.
  const bitmap = await createImageBitmap(file, {
    imageOrientation: "from-image",
  });
  try {
    const scale = Math.min(1, MAX_EDGE / Math.max(bitmap.width, bitmap.height));
    const width = Math.round(bitmap.width * scale);
    const height = Math.round(bitmap.height * scale);

    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("no 2d canvas context");
    ctx.drawImage(bitmap, 0, 0, width, height);

    let blob = await toBlob(canvas, 0.8);
    for (const quality of [0.6, 0.45]) {
      if (blob.size <= TARGET_BYTES) break;
      blob = await toBlob(canvas, quality);
    }
    return blob;
  } finally {
    bitmap.close();
  }
}

function toBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error("toBlob failed"))),
      "image/jpeg",
      quality,
    );
  });
}
