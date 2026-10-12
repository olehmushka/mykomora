import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Notice } from "@/components/notice";
import { getInvitePreview } from "@/lib/api/family";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Join");
  return { title: t("metaTitle") };
}

/**
 * Where an invite link lands.
 *
 * The invitee has no account yet, so this page is public and the preview it
 * shows is deliberately thin: the family's name, and whether the link still
 * works. An invite is enough to see who invited you and never enough to see
 * what they own.
 */
export default async function JoinPage(props: PageProps<"/join/[token]">) {
  const t = await getTranslations("Join");
  const { token } = await props.params;
  const preview = await getInvitePreview(token);

  if (!preview.ok) {
    return (
      <Shell>
        <Notice tone="warning" title={t("missingTitle")}>
          {t("missingDetail")}
        </Notice>
      </Shell>
    );
  }

  if (!preview.data.usable) {
    return (
      <Shell>
        <Notice tone="warning" title={t("unusableTitle", { family: preview.data.familyName })}>
          {preview.data.status === "accepted" ? t("usedDetail") : t("expiredDetail")}
        </Notice>
      </Shell>
    );
  }

  return (
    <Shell>
      <h1 className="text-2xl font-semibold tracking-tight">
        {t("title", { family: preview.data.familyName })}
      </h1>
      <p className="mt-2 text-sm text-muted">{t("intro")}</p>

      <Link
        href={`/api/v1/auth/google/start?invite=${encodeURIComponent(token)}`}
        prefetch={false}
        className="mt-8 inline-flex min-h-11 items-center justify-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
      >
        {t("continue")}
      </Link>
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center px-4 py-12 sm:px-6">
      {children}
    </main>
  );
}
