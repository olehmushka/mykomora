"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { Input } from "@/components/ui/input";

const FORMATS = ["ean_13", "ean_8", "upc_a", "upc_e"] as const;
const ZXING_FORMATS = ["EAN-13", "EAN-8", "UPC-A", "UPC-E"] as const;
const SCAN_INTERVAL_MS = 250;

type Detector = { detect(source: CanvasImageSource): Promise<{ rawValue: string }[]> };

declare global {
  interface Window {
    BarcodeDetector?: new (options: { formats: readonly string[] }) => Detector;
  }
}

/**
 * Builds a function that finds a barcode in one video frame: the browser's
 * BarcodeDetector where it exists, otherwise ZXing compiled to WASM, loaded
 * only then so most phones never download it.
 */
async function makeReader(): Promise<(video: HTMLVideoElement) => Promise<string | null>> {
  if (window.BarcodeDetector) {
    const detector = new window.BarcodeDetector({ formats: FORMATS });

    return async (video) => (await detector.detect(video))[0]?.rawValue ?? null;
  }

  const { readBarcodes } = await import("zxing-wasm/reader");
  const canvas = document.createElement("canvas");

  return async (video) => {
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;

    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx || canvas.width === 0) return null;

    ctx.drawImage(video, 0, 0);
    const results = await readBarcodes(ctx.getImageData(0, 0, canvas.width, canvas.height), {
      formats: [...ZXING_FORMATS],
      maxNumberOfSymbols: 1,
    });

    return results[0]?.text ?? null;
  };
}

/**
 * Camera barcode scanner with a typed-code fallback (also what a USB or
 * Bluetooth scanner types into). `onCode` fires once per distinct code in a
 * row, so holding the camera on a barcode does not repeat it.
 */
export function BarcodeScanner({
  onCode,
  onClose,
}: {
  onCode: (code: string) => void;
  onClose: () => void;
}) {
  const t = useTranslations("Scanner");
  const videoRef = useRef<HTMLVideoElement>(null);
  const [cameraError, setCameraError] = useState(false);
  const onCodeRef = useRef(onCode);

  useEffect(() => {
    onCodeRef.current = onCode;
  }, [onCode]);

  useEffect(() => {
    let stopped = false;
    let stream: MediaStream | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let last = "";

    async function start() {
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: "environment" },
        });
        const video = videoRef.current;
        if (stopped || !video) return;

        video.srcObject = stream;
        await video.play();

        const read = await makeReader();

        const tick = async () => {
          if (stopped) return;

          try {
            const code = await read(video);
            if (code && code !== last) {
              last = code;
              onCodeRef.current(code);
            }
          } catch {
            // A frame that cannot be decoded is just a frame with no barcode.
          }

          timer = setTimeout(() => void tick(), SCAN_INTERVAL_MS);
        };

        void tick();
      } catch {
        if (!stopped) setCameraError(true);
      }
    }

    void start();

    return () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      stream?.getTracks().forEach((track) => track.stop());
    };
  }, []);

  return (
    <div className="rounded-md border border-border p-3">
      {cameraError ? (
        <p role="alert" className="text-sm text-muted">
          {t("cameraUnavailable")}
        </p>
      ) : (
        <video ref={videoRef} playsInline muted className="max-h-64 w-full rounded-md bg-black" />
      )}

      <form
        className="mt-2 flex gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          const input = event.currentTarget.elements.namedItem("manualCode") as HTMLInputElement;
          const code = input.value.trim();
          if (code) {
            onCode(code);
            input.value = "";
          }
        }}
      >
        <Input
          name="manualCode"
          inputMode="numeric"
          aria-label={t("manualLabel")}
          placeholder={t("manualLabel")}
        />
        <button
          type="submit"
          className="mt-1 min-h-11 rounded-md border border-border px-4 text-sm hover:bg-border/60"
        >
          {t("manualSubmit")}
        </button>
      </form>

      <button
        type="button"
        onClick={onClose}
        className="mt-2 inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm hover:bg-border/60"
      >
        {t("close")}
      </button>
    </div>
  );
}
