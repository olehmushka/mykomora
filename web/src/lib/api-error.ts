import { getTranslations } from "next-intl/server";

import type { ApiFailure } from "@/lib/api/client";

/**
 * A failed API call as a sentence in the person's language.
 *
 * "Unreachable" and the codes that name a generic situation are translated
 * here. Any other code falls back to core-api's own message, which is more
 * specific than a catalogue entry could be but is English only.
 */
export async function apiError(failure: Pick<ApiFailure, "reason" | "detail" | "code">) {
  const t = await getTranslations("Errors");

  if (failure.reason === "unavailable") {
    return t("unavailable");
  }

  if (failure.reason === "forbidden") {
    return t("forbidden");
  }

  if (failure.code && t.has(`code.${failure.code}`)) {
    return t(`code.${failure.code}`);
  }

  return failure.detail;
}

/** A catalogue message from the `Errors` namespace, for validation failures. */
export async function errorText(key: string, values?: Record<string, string | number>) {
  const t = await getTranslations("Errors");

  return t(key, values);
}
