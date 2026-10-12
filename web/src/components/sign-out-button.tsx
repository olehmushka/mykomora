import { useTranslations } from "next-intl";

import { signOut } from "@/app/actions";

/**
 * Sign-out is a form, not a link: it changes server state, and a GET that
 * changes state is something a browser is entitled to prefetch.
 */
export function SignOutButton() {
  const t = useTranslations("AppShell");

  return (
    <form action={signOut} className="ml-auto sm:ml-0">
      <button
        type="submit"
        className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
      >
        {t("signOut")}
      </button>
    </form>
  );
}
