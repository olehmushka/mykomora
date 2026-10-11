/**
 * Client-side resize before upload (SPEC 6, "Photos and attachments": the
 * client resizes to a 1600px long edge and generates a 320px thumbnail
 * before anything is uploaded). Browser-only — every API here
 * (createImageBitmap, canvas) needs a DOM, so this is never imported from
 * a server component.
 */
export const PHOTO_MAX_EDGE = 1600;
export const THUMB_MAX_EDGE = 320;
const JPEG_QUALITY = 0.85;
export const RESIZED_CONTENT_TYPE = "image/jpeg";

export type ResizedImage = { blob: Blob; width: number; height: number };

export async function resizeImage(file: File, maxEdge: number): Promise<ResizedImage> {
  const bitmap = await createImageBitmap(file);

  try {
    const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height));
    const width = Math.max(1, Math.round(bitmap.width * scale));
    const height = Math.max(1, Math.round(bitmap.height * scale));

    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;

    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("this browser cannot resize images");

    ctx.drawImage(bitmap, 0, 0, width, height);

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, RESIZED_CONTENT_TYPE, JPEG_QUALITY),
    );
    if (!blob) throw new Error("could not encode the resized image");

    return { blob, width, height };
  } finally {
    bitmap.close();
  }
}
