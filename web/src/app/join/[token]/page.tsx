import Link from "next/link";

import { Notice } from "@/components/notice";
import { getInvitePreview } from "@/lib/api/family";

export const metadata = { title: "Join a family — mykomora" };

/**
 * Where an invite link lands.
 *
 * The invitee has no account yet, so this page is public and the preview it
 * shows is deliberately thin: the family's name, and whether the link still
 * works. An invite is enough to see who invited you and never enough to see
 * what they own.
 */
export default async function JoinPage(props: PageProps<"/join/[token]">) {
  const { token } = await props.params;
  const preview = await getInvitePreview(token);

  if (!preview.ok) {
    return (
      <Shell>
        <Notice tone="warning" title="This invite link does not exist">
          Check that you copied the whole link, or ask whoever invited you for a new one.
        </Notice>
      </Shell>
    );
  }

  if (!preview.data.usable) {
    return (
      <Shell>
        <Notice
          tone="warning"
          title={`This invite to ${preview.data.familyName} is no longer usable`}
        >
          {preview.data.status === "accepted"
            ? "It has already been used — an invite admits one person."
            : "It has expired or been cancelled. Ask for a new link."}
        </Notice>
      </Shell>
    );
  }

  return (
    <Shell>
      <h1 className="text-2xl font-semibold tracking-tight">Join {preview.data.familyName}</h1>
      <p className="mt-2 text-sm text-muted">
        Sign in with Google and you will share this family&apos;s inventory. Everyone in a family
        sees and edits everything.
      </p>

      <Link
        href={`/api/v1/auth/google/start?invite=${encodeURIComponent(token)}`}
        prefetch={false}
        className="mt-8 inline-flex min-h-11 items-center justify-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60"
      >
        Continue with Google
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
