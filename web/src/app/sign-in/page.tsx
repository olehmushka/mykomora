import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { LocaleSwitcher } from "@/components/locale-switcher";
import { Notice } from "@/components/notice";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("SignIn");
  return { title: t("metaTitle") };
}

/** Codes core-api sets when it redirects here after a failed sign-in. */
const REASONS = ["denied", "expired", "unverified_email", "exchange_failed", "unavailable"];

export default async function SignInPage(props: PageProps<"/sign-in">) {
  const t = await getTranslations("SignIn");
  const params = await props.searchParams;
  const reason =
    typeof params.error === "string" && REASONS.includes(params.error) ? params.error : undefined;
  const next = typeof params.next === "string" ? params.next : undefined;
  const invite = typeof params.invite === "string" ? params.invite : undefined;

  const query = new URLSearchParams();
  if (next) {
    query.set("next", next);
  }
  if (invite) {
    query.set("invite", invite);
  }

  const start = `/api/v1/auth/google/start${query.size > 0 ? `?${query}` : ""}`;

  return (
    <main className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center px-4 py-12 sm:px-6">
      <div className="self-end">
        <LocaleSwitcher />
      </div>

      {/* eslint-disable-next-line i18next/no-literal-string -- the product name */}
      <h1 className="text-2xl font-semibold tracking-tight">mykomora</h1>
      <p className="mt-2 text-sm text-muted">{t("tagline")}</p>

      {reason && (
        <div className="mt-6">
          <Notice tone="warning" title={t(`reason.${reason}.title`)}>
            {t(`reason.${reason}.detail`)}
          </Notice>
        </div>
      )}

      {/*
        A plain link, not a fetch: signing in is a top-level navigation to
        Google and back, and core-api sets the session cookies on the redirect
        it answers with.
      */}
      <Link
        href={start}
        prefetch={false}
        className="mt-8 inline-flex min-h-11 items-center justify-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
      >
        {t("continue")}
      </Link>

      <p className="mt-6 text-xs text-muted">{t("privacy")}</p>
    </main>
  );
}
