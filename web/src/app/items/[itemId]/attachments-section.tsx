"use client";

import { useRef, useState } from "react";
import { useTranslations } from "next-intl";

import {
  confirmAttachment,
  deleteAttachment,
  presignUploadAction,
  refreshAttachments,
} from "@/app/items/[itemId]/actions";
import { Notice } from "@/components/notice";
import { Select } from "@/components/ui/select";
import type { AttachmentKind, ItemAttachment } from "@/lib/api/attachments";
import { UploadError } from "@/lib/upload-error";

const KINDS: AttachmentKind[] = ["receipt", "manual", "warranty", "other"];

/** Receipts, manuals and other documents — no resize, no thumbnail, just a direct upload (SPEC 6). */
export function AttachmentsSection({
  itemId,
  attachments: initial,
}: {
  itemId: string;
  attachments: ItemAttachment[];
}) {
  const t = useTranslations("ItemDetail");
  const tError = useTranslations("Errors");
  const [attachments, setAttachments] = useState(initial);
  const [kind, setKind] = useState<AttachmentKind>("receipt");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const inputRef = useRef<HTMLInputElement>(null);

  async function refresh() {
    const result = await refreshAttachments(itemId);
    if (result.ok) setAttachments(result.data);
  }

  async function handleFile(file: File | undefined) {
    if (!file) return;

    setBusy(true);
    setError(undefined);

    try {
      const presign = await presignUploadAction({
        itemId,
        purpose: "attachment",
        contentType: file.type,
        contentLength: file.size,
      });
      if (!presign.ok) throw new Error(presign.detail);

      const putResponse = await fetch(presign.data.uploadUrl, {
        method: "PUT",
        headers: presign.data.uploadHeaders,
        body: file,
      });
      if (!putResponse.ok) throw new UploadError(putResponse.status);

      const confirmed = await confirmAttachment(itemId, {
        storageKey: presign.data.storageKey,
        filename: file.name,
        contentType: file.type,
        bytes: file.size,
        kind,
      });
      if (!confirmed.ok) throw new Error(confirmed.detail);

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

  async function handleDelete(attachmentId: string) {
    if (!window.confirm(t("confirmDeleteAttachment"))) return;

    setBusy(true);
    setError(undefined);

    const result = await deleteAttachment(itemId, attachmentId);
    if (!result.ok) setError(result.detail);

    await refresh();
    setBusy(false);
  }

  return (
    <div className="mt-3">
      {attachments.length === 0 && <Notice>{t("noAttachments")}</Notice>}

      {attachments.length > 0 && (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {attachments.map((attachment) => (
            <li key={attachment.id} className="flex items-center justify-between gap-3 px-4 py-3">
              <a
                href={attachment.url}
                target="_blank"
                rel="noreferrer"
                className="truncate text-sm font-medium underline underline-offset-4"
              >
                {attachment.filename}
              </a>
              <div className="flex items-center gap-3">
                <span className="text-sm text-muted">{t(`attachmentKind.${attachment.kind}`)}</span>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => handleDelete(attachment.id)}
                  className="min-h-11 rounded-md border border-border px-3 text-sm text-degraded hover:bg-border/60 disabled:opacity-40"
                >
                  {t("deleteAttachmentButton")}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <div className="mt-3 flex flex-wrap items-end gap-3">
        <Select
          value={kind}
          onChange={(event) => setKind(event.target.value as AttachmentKind)}
          disabled={busy}
          className="mt-0 w-auto"
          aria-label={t("attachmentKindLabel")}
        >
          {KINDS.map((k) => (
            <option key={k} value={k}>
              {t(`attachmentKind.${k}`)}
            </option>
          ))}
        </Select>

        <label className="inline-flex min-h-11 cursor-pointer items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60">
          {busy ? t("uploadingAttachment") : t("addAttachmentButton")}
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png,application/pdf"
            disabled={busy}
            onChange={(event) => handleFile(event.target.files?.[0])}
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
