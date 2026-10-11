import Link from "next/link";
import { useTranslations } from "next-intl";

import { LocaleSwitcher } from "@/components/locale-switcher";
import { SignOutButton } from "@/components/sign-out-button";
import type { Me } from "@/lib/api/session";

/**
 * The frame every signed-in page sits in.
 *
 * Mobile-first: the navigation is a single scrollable row rather than a menu,
 * because at this size there are four destinations and a hamburger would hide
 * them behind a tap for no reason.
 */
export function AppShell({ me, children }: { me: Me; children: React.ReactNode }) {
  const t = useTranslations("AppShell");

  return (
    <>
      <header className="border-b border-border">
        <div className="mx-auto flex w-full max-w-3xl flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:px-6">
          <Link href="/" className="text-base font-semibold tracking-tight">
            mykomora
          </Link>
          <span className="text-sm text-muted">{me.family.name}</span>

          <nav
            aria-label="Main"
            className="-mx-2 order-last w-full overflow-x-auto sm:order-none sm:ml-auto sm:w-auto sm:overflow-visible"
          >
            <ul className="flex items-center gap-1">
              <NavItem href="/">{t("inventory")}</NavItem>
              <NavItem href="/items">{t("items")}</NavItem>
              <NavItem href="/people">{t("people")}</NavItem>
              <NavItem href="/locations">{t("locations")}</NavItem>
              <NavItem href="/settings/taxonomy">{t("taxonomy")}</NavItem>
              <NavItem href="/settings/tags">{t("tags")}</NavItem>
              <NavItem href="/settings/family">{t("family")}</NavItem>
            </ul>
          </nav>

          <LocaleSwitcher />
          <SignOutButton />
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-8 sm:px-6 sm:py-12">{children}</main>
    </>
  );
}

function NavItem({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <li>
      <Link
        href={href}
        className="inline-flex min-h-11 items-center rounded-md px-3 text-sm hover:bg-border/60"
      >
        {children}
      </Link>
    </li>
  );
}
