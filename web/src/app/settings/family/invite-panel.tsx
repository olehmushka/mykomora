"use client";

import { useTranslations } from "next-intl";
import { useActionState } from "react";

import { createInvite, type ActionState } from "@/app/settings/family/actions";

/**
 * The invite issuer.
 *
 * A client component for one reason: the link comes back in the action's
 * result and has to survive on screen afterwards. It cannot be re-fetched —
 * core-api stores only a digest of the token — so if this were a plain form
 * post the link would be gone before anyone could copy it.
 */
export function InvitePanel() {
  const t = useTranslations("Family");
  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async () => createInvite(),
    {},
  );

  return (
    <div className="mt-3 rounded-lg border border-border px-4 py-4">
      <form action={formAction}>
        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? t("creating") : t("createInvite")}
        </button>
      </form>

      {state.error && (
        <p role="alert" className="mt-3 text-sm text-degraded">
          {state.error}
        </p>
      )}

      {state.inviteUrl && (
        <div className="mt-4">
          <label htmlFor="invite-url" className="text-sm font-medium">
            {t("shareLink")}
          </label>
          <p className="mt-1 text-sm text-muted">{t("shareLinkNote")}</p>
          <input
            id="invite-url"
            readOnly
            value={state.inviteUrl}
            onFocus={(event) => event.currentTarget.select()}
            className="mt-2 w-full rounded-md border border-border bg-transparent px-3 py-2 font-mono text-xs"
          />
        </div>
      )}
    </div>
  );
}
