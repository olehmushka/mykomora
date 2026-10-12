"use client";

import { useRef, useState } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";

import {
  confirmPhoto,
  deletePhoto,
  presignUploadAction,
  refreshPhotos,
  updatePhoto,
} from "@/app/items/[itemId]/actions";
import { Notice } from "@/components/notice";
import type { ItemPhoto } from "@/lib/api/photos";
import type { PresignUploadResult } from "@/lib/api/uploads";
import { UploadError } from "@/lib/upload-error";
import {
  PHOTO_MAX_EDGE,
  RESIZED_CONTENT_TYPE,
  THUMB_MAX_EDGE,
  resizeImage,
} from "@/lib/image-resize";

const THUMB_PX = 112;

/**
 * Photo gallery: upload (with client-side resize), primary selection and
 * reordering (SPEC 6, "Photos and attachments"). Reordering is two
 * "move left/right" buttons rather than drag-and-drop — no drag library
 * exists in this app yet, and arrow buttons are also the more accessible
 * choice for a touch-first, mobile-browser flow.
 */
export function PhotoGallery({ itemId, photos: initial }: { itemId: string; photos: ItemPhoto[] }) {
  const t = useTranslations("ItemDetail");
  const tError = useTranslations("Errors");
  const [photos, setPhotos] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const inputRef = useRef<HTMLInputElement>(null);

  async function refresh() {
    const result = await refreshPhotos(itemId);
    if (result.ok) setPhotos(result.data);
  }

  async function uploadOne(file: File) {
    const full = await resizeImage(file, PHOTO_MAX_EDGE);
    const thumb = await resizeImage(file, THUMB_MAX_EDGE);

    const fullPresign = await presignUploadAction({
      itemId,
      purpose: "photo",
      contentType: RESIZED_CONTENT_TYPE,
      contentLength: full.blob.size,
    });
    if (!fullPresign.ok) throw new Error(fullPresign.detail);

    const thumbPresign = await presignUploadAction({
      itemId,
      purpose: "photo_thumb",
      contentType: RESIZED_CONTENT_TYPE,
      contentLength: thumb.blob.size,
    });
    if (!thumbPresign.ok) throw new Error(thumbPresign.detail);

    await putObject(fullPresign.data, full.blob);
    await putObject(thumbPresign.data, thumb.blob);

    const confirmed = await confirmPhoto(itemId, {
      storageKey: fullPresign.data.storageKey,
      thumbStorageKey: thumbPresign.data.storageKey,
      width: full.width,
      height: full.height,
      bytes: full.blob.size,
      contentType: RESIZED_CONTENT_TYPE,
      // The server always makes the first photo primary regardless of this
      // flag; false just means "no request to also take over from an
      // existing primary".
      isPrimary: false,
    });
    if (!confirmed.ok) throw new Error(confirmed.detail);
  }

  async function handleFiles(fileList: FileList | null) {
    if (!fileList || fileList.length === 0) return;

    setBusy(true);
    setError(undefined);

    try {
      for (const file of Array.from(fileList)) {
        await uploadOne(file);
      }

      await refresh();
    } catch (err) {
      setError(
        err instanceof UploadError
          ? tError("uploadFailed", { status: err.status })
          : err instanceof Error
            ? err.message
            : String(err),
      );
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function handleSetPrimary(photoId: string) {
    setBusy(true);
    setError(undefined);

    const result = await updatePhoto(itemId, photoId, { isPrimary: true });
    if (!result.ok) setError(result.detail);

    await refresh();
    setBusy(false);
  }

  async function handleMove(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= photos.length) return;

    setBusy(true);
    setError(undefined);

    const a = photos[index];
    const b = photos[target];
    if (!a || !b) return;

    const results = await Promise.all([
      updatePhoto(itemId, a.id, { sortOrder: b.sortOrder }),
      updatePhoto(itemId, b.id, { sortOrder: a.sortOrder }),
    ]);
    const failed = results.find((r) => !r.ok);
    if (failed && !failed.ok) setError(failed.detail);

    await refresh();
    setBusy(false);
  }

  async function handleDelete(photoId: string) {
    if (!window.confirm(t("confirmDeletePhoto"))) return;

    setBusy(true);
    setError(undefined);

    const result = await deletePhoto(itemId, photoId);
    if (!result.ok) setError(result.detail);

    await refresh();
    setBusy(false);
  }

  return (
    <div className="mt-3">
      {photos.length === 0 && <Notice>{t("noPhotos")}</Notice>}

      {photos.length > 0 && (
        <ul className="flex flex-wrap gap-3">
          {photos.map((photo, index) => (
            <li key={photo.id} className="w-28">
              <div className="relative overflow-hidden rounded-lg border border-border">
                <Image
                  src={photo.thumbUrl}
                  alt=""
                  unoptimized
                  width={THUMB_PX}
                  height={THUMB_PX}
                  className="aspect-square w-full object-cover"
                />
                {photo.isPrimary && (
                  <span className="absolute left-1 top-1 rounded-full bg-background/90 px-2 py-0.5 text-xs font-medium">
                    {t("primaryBadge")}
                  </span>
                )}
              </div>

              <div className="mt-1 flex items-center justify-between gap-1">
                <button
                  type="button"
                  disabled={busy || index === 0}
                  onClick={() => handleMove(index, -1)}
                  aria-label={t("moveLeftButton")}
                  className="min-h-8 min-w-8 rounded-md border border-border text-xs disabled:opacity-40"
                >
                  ←
                </button>

                {!photo.isPrimary && (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => handleSetPrimary(photo.id)}
                    className="min-h-8 flex-1 truncate rounded-md border border-border px-1 text-xs hover:bg-border/60 disabled:opacity-40"
                  >
                    {t("makePrimaryButton")}
                  </button>
                )}

                <button
                  type="button"
                  disabled={busy || index === photos.length - 1}
                  onClick={() => handleMove(index, 1)}
                  aria-label={t("moveRightButton")}
                  className="min-h-8 min-w-8 rounded-md border border-border text-xs disabled:opacity-40"
                >
                  →
                </button>
              </div>

              <button
                type="button"
                disabled={busy}
                onClick={() => handleDelete(photo.id)}
                className="mt-1 min-h-8 w-full rounded-md border border-border text-xs text-degraded hover:bg-border/60 disabled:opacity-40"
              >
                {t("deletePhotoButton")}
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="mt-3">
        <label className="inline-flex min-h-11 cursor-pointer items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60">
          {busy ? t("uploadingPhoto") : t("addPhotoButton")}
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            capture="environment"
            multiple
            disabled={busy}
            onChange={(event) => handleFiles(event.target.files)}
            className="sr-only"
          />
        </label>
      </div>

      {error && (
        <p role="alert" className="mt-2 text-sm text-degraded">
          {error}
        </p>
      )}
    </div>
  );
}

async function putObject(presign: PresignUploadResult, blob: Blob) {
  const response = await fetch(presign.uploadUrl, {
    method: "PUT",
    headers: presign.uploadHeaders,
    body: blob,
  });

  if (!response.ok) {
    throw new UploadError(response.status);
  }
}
