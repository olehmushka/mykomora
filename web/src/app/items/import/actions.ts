"use server";

import { revalidatePath } from "next/cache";

import {
  commitImport,
  dryRunImport,
  inspectImport,
  presignImport,
  type ImportInspection,
  type ImportPresign,
  type ImportReport,
  type ImportRequest,
  type ImportResult,
} from "@/lib/api/imports";

/** Called straight from the client wizard, so failures are values, not throws. */
export type Outcome<T> = { ok: true; data: T } | { ok: false; error: string };

type ApiResult<T> = { ok: true; data: T } | { ok: false; reason: string; detail: string };

function outcome<T>(result: ApiResult<T>): Outcome<T> {
  if (result.ok) return { ok: true, data: result.data };

  return {
    ok: false,
    error: result.reason === "unavailable" ? "unavailable" : result.detail,
  };
}

export async function presignImportAction(
  contentType: string,
  contentLength: number,
): Promise<Outcome<ImportPresign>> {
  return outcome(await presignImport(contentType, contentLength));
}

export async function inspectImportAction(storageKey: string): Promise<Outcome<ImportInspection>> {
  return outcome(await inspectImport(storageKey));
}

export async function dryRunImportAction(body: ImportRequest): Promise<Outcome<ImportReport>> {
  return outcome(await dryRunImport(body));
}

export async function commitImportAction(body: ImportRequest): Promise<Outcome<ImportResult>> {
  const result = outcome(await commitImport(body));

  if (result.ok) revalidatePath("/items");

  return result;
}
