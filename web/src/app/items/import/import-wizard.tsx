"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import {
  commitImportAction,
  dryRunImportAction,
  inspectImportAction,
  presignImportAction,
  type Outcome,
} from "@/app/items/import/actions";
import type { FlatCategory, LocationOption } from "@/app/items/new/item-form";
import { FieldRow } from "@/components/ui/field-row";
import { Select } from "@/components/ui/select";
import type { ImportInspection, ImportReport, ImportRequest } from "@/lib/api/imports";

const MAX_BYTES = 5 * 1024 * 1024;

type Step = "upload" | "map" | "report";

export function ImportWizard({
  categories,
  locations,
}: {
  categories: FlatCategory[];
  locations: LocationOption[];
}) {
  const t = useTranslations("Import");
  const router = useRouter();

  const [step, setStep] = useState<Step>("upload");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [storageKey, setStorageKey] = useState("");
  const [inspection, setInspection] = useState<ImportInspection>();
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [categoryId, setCategoryId] = useState("");
  const [locationId, setLocationId] = useState("");
  const [report, setReport] = useState<ImportReport>();

  // Surfaces a failed call; returns the data when it worked.
  function take<T>(result: Outcome<T>): T | undefined {
    if (result.ok) return result.data;

    setError(result.error === "unavailable" ? t("unavailable") : result.error);

    return undefined;
  }

  async function upload(file: File) {
    setError("");

    if (file.size === 0 || file.size > MAX_BYTES) {
      setError(t("badSize"));

      return;
    }

    setBusy(true);

    try {
      const contentType = file.type || "text/csv";
      const presign = take(await presignImportAction(contentType, file.size));
      if (!presign) return;

      const put = await fetch(presign.uploadUrl, {
        method: "PUT",
        headers: presign.uploadHeaders,
        body: file,
      });
      if (!put.ok) throw new Error(`upload failed (${put.status})`);

      const inspected = take(await inspectImportAction(presign.storageKey));
      if (!inspected) return;

      setStorageKey(presign.storageKey);
      setInspection(inspected);
      setMapping(inspected.suggestedMapping);
      setStep("map");
    } catch (e) {
      setError(e instanceof Error ? e.message : t("unavailable"));
    } finally {
      setBusy(false);
    }
  }

  function request(): ImportRequest {
    const defaults: NonNullable<ImportRequest["defaults"]> = {};
    if (categoryId) defaults.categoryId = categoryId;
    if (locationId) defaults.locationId = locationId;

    return { storageKey, mapping, defaults };
  }

  async function check() {
    setError("");
    setBusy(true);

    const result = take(await dryRunImportAction(request()));
    setBusy(false);

    if (result) {
      setReport(result);
      setStep("report");
    }
  }

  async function commit() {
    setError("");
    setBusy(true);

    const result = take(await commitImportAction(request()));
    setBusy(false);

    if (result) router.push("/items");
  }

  const nameMapped = Boolean(mapping["name"]);

  return (
    <div className="mt-4 flex flex-col gap-6">
      {error && (
        <p role="alert" className="text-sm text-red-600">
          {error}
        </p>
      )}

      {step === "upload" && (
        <section>
          <p className="text-sm text-muted">{t("uploadHelp")}</p>
          <label className="mt-3 inline-flex min-h-11 cursor-pointer items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60">
            {busy ? t("uploading") : t("chooseFile")}
            <input
              type="file"
              accept=".csv,text/csv"
              className="sr-only"
              disabled={busy}
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void upload(file);
              }}
            />
          </label>
        </section>
      )}

      {step === "map" && inspection && (
        <section className="flex flex-col gap-4">
          <p className="text-sm text-muted">{t("rowsFound", { count: inspection.rowCount })}</p>

          <h2 className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("mappingHeading")}
          </h2>

          <div className="grid gap-3 sm:grid-cols-2">
            {inspection.fields.map((field) => (
              <FieldRow key={field} label={t(`field.${field}`)} htmlFor={`map-${field}`}>
                <Select
                  id={`map-${field}`}
                  value={mapping[field] ?? ""}
                  onChange={(event) => {
                    const value = event.target.value;
                    setMapping((current) => {
                      const next = { ...current };
                      if (value) next[field] = value;
                      else delete next[field];

                      return next;
                    });
                  }}
                >
                  <option value="">{t("notMapped")}</option>
                  {inspection.headers.map((header) => (
                    <option key={header} value={header}>
                      {header}
                    </option>
                  ))}
                </Select>
              </FieldRow>
            ))}
          </div>

          <h2 className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("defaultsHeading")}
          </h2>
          <p className="text-sm text-muted">{t("defaultsHelp")}</p>

          <div className="grid gap-3 sm:grid-cols-2">
            <FieldRow label={t("defaultCategory")} htmlFor="default-category">
              <Select
                id="default-category"
                value={categoryId}
                onChange={(event) => setCategoryId(event.target.value)}
              >
                <option value="">{t("noDefault")}</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {"  ".repeat(c.depth)}
                    {c.name}
                  </option>
                ))}
              </Select>
            </FieldRow>

            <FieldRow label={t("defaultLocation")} htmlFor="default-location">
              <Select
                id="default-location"
                value={locationId}
                onChange={(event) => setLocationId(event.target.value)}
              >
                <option value="">{t("noDefault")}</option>
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </Select>
            </FieldRow>
          </div>

          {!nameMapped && <p className="text-sm text-muted">{t("needName")}</p>}

          <button
            type="button"
            disabled={busy || !nameMapped}
            onClick={() => void check()}
            className="inline-flex min-h-11 items-center justify-center rounded-md bg-foreground px-4 text-sm font-medium text-background disabled:opacity-60 sm:self-start"
          >
            {t("checkButton")}
          </button>
        </section>
      )}

      {step === "report" && report && (
        <section className="flex flex-col gap-4">
          <p className="text-sm">
            {t("summary", { valid: report.validRows, total: report.totalRows })}
          </p>

          {report.errors.length > 0 && (
            <div>
              <h2 className="text-sm font-medium uppercase tracking-wide text-muted">
                {t("problemsHeading")}
              </h2>
              <ul className="mt-2 flex flex-col gap-1 text-sm">
                {report.errors.map((e) => (
                  <li key={`${e.row}-${e.message}`}>
                    {t("rowError", { row: e.row, message: e.message })}
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-sm text-muted">{t("fixAndRetry")}</p>
            </div>
          )}

          {report.preview.length > 0 && (
            <div>
              <h2 className="text-sm font-medium uppercase tracking-wide text-muted">
                {t("previewHeading")}
              </h2>
              <ul className="mt-2 text-sm">
                {report.preview.map((p) => (
                  <li key={p.row}>{p.name}</li>
                ))}
              </ul>
            </div>
          )}

          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setStep("map")}
              className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
            >
              {t("backToMapping")}
            </button>
            <button
              type="button"
              disabled={busy || report.errors.length > 0 || report.validRows === 0}
              onClick={() => void commit()}
              className="inline-flex min-h-11 items-center justify-center rounded-md bg-foreground px-4 text-sm font-medium text-background disabled:opacity-60"
            >
              {t("commitButton", { count: report.validRows })}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
