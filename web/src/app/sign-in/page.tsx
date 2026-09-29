import Link from "next/link";

import { Notice } from "@/components/notice";

export const metadata = { title: "Sign in — mykomora" };

/**
 * Why a sign-in attempt did not work, in the words the person needs.
 *
 * The codes are set by core-api, which redirects here rather than answering a
 * browser with JSON. These strings move to the message catalogue in M2; until
 * then English is the only locale the app has.
 */
const REASONS: Record<string, { title: string; detail: string }> = {
  denied: {
    title: "Sign-in was cancelled",
    detail: "You did not grant access on Google's screen. Nothing was shared.",
  },
  expired: {
    title: "That sign-in took too long",
    detail: "The link expired or was already used. Start again below.",
  },
  unverified_email: {
    title: "That Google account has no verified email",
    detail:
      "A family recognises its members by their address, so an unverified one cannot be used.",
  },
  exchange_failed: {
    title: "Google did not complete the sign-in",
    detail: "Something went wrong between here and Google. Trying again usually works.",
  },
  unavailable: {
    title: "Sign-in is not set up on this deployment",
    detail:
      "This copy of mykomora has no Google OAuth client configured. See “Google sign-in” in the README.",
  },
};

export default async function SignInPage(props: PageProps<"/sign-in">) {
  const params = await props.searchParams;
  const reason = typeof params.error === "string" ? REASONS[params.error] : undefined;
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
      <h1 className="text-2xl font-semibold tracking-tight">mykomora</h1>
      <p className="mt-2 text-sm text-muted">
        A family inventory for households spread across several places.
      </p>

      {reason && (
        <div className="mt-6">
          <Notice tone="warning" title={reason.title}>
            {reason.detail}
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
        Continue with Google
      </Link>

      <p className="mt-6 text-xs text-muted">
        Google is the only way in. mykomora never sees your password, and stores only your name,
        email address and profile picture.
      </p>
    </main>
  );
}
