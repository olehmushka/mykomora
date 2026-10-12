import type { Metadata } from "next";
import { getLocale, getTranslations } from "next-intl/server";

import { AppShell } from "@/components/app-shell";
import { Notice } from "@/components/notice";
import { removeMember, revokeInvite } from "@/app/settings/family/actions";
import { InvitePanel } from "@/app/settings/family/invite-panel";
import { RenameForm } from "@/app/settings/family/rename-form";
import { listFamilyInvites, listFamilyMembers, type FamilyInvite } from "@/lib/api/family";
import { requireMe } from "@/lib/api/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Family");
  return { title: t("metaTitle") };
}

export default async function FamilySettingsPage() {
  const t = await getTranslations("Family");
  const locale = await getLocale();
  const me = await requireMe("/settings/family");
  const isOwner = me.role === "owner";

  const [members, invites] = await Promise.all([
    listFamilyMembers(),
    // Only owners may list invites, so a member's page simply does not ask.
    isOwner ? listFamilyInvites() : Promise.resolve(null),
  ]);

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{t("title")}</h1>
      <p className="mt-2 text-sm text-muted">{t("intro")}</p>

      <section aria-labelledby="name" className="mt-10">
        <h2 id="name" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("nameHeading")}
        </h2>
        {isOwner ? (
          <RenameForm currentName={me.family.name} />
        ) : (
          <p className="mt-3 text-sm">{me.family.name}</p>
        )}
      </section>

      <section aria-labelledby="members" className="mt-10">
        <h2 id="members" className="text-sm font-medium uppercase tracking-wide text-muted">
          {t("membersHeading")}
        </h2>

        {members.ok ? (
          <ul className="mt-3 divide-y divide-border rounded-lg border border-border">
            {members.data.map((member) => (
              <li
                key={member.userId}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{member.displayName}</p>
                  <p className="truncate text-sm text-muted">{member.email}</p>
                </div>

                <div className="flex items-center gap-3">
                  <span className="text-xs uppercase tracking-wide text-muted">
                    {t(`role.${member.role}`)}
                  </span>

                  {isOwner && member.role !== "owner" && (
                    <form action={removeMember}>
                      <input type="hidden" name="userId" value={member.userId} />
                      <button
                        type="submit"
                        className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
                      >
                        {t("remove")}
                      </button>
                    </form>
                  )}
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <div className="mt-3">
            <Notice tone="warning" title={t("membersUnavailable")}>
              {members.detail}
            </Notice>
          </div>
        )}
      </section>

      {isOwner && (
        <section aria-labelledby="invites" className="mt-10">
          <h2 id="invites" className="text-sm font-medium uppercase tracking-wide text-muted">
            {t("invitesHeading")}
          </h2>
          <p className="mt-3 text-sm text-muted">{t("invitesIntro")}</p>

          <InvitePanel />

          {invites?.ok && invites.data.length > 0 && (
            <ul className="mt-4 divide-y divide-border rounded-lg border border-border">
              {invites.data.map((invite) => (
                <InviteRow key={invite.id} invite={invite} locale={locale} />
              ))}
            </ul>
          )}
        </section>
      )}
    </AppShell>
  );
}

async function InviteRow({ invite, locale }: { invite: FamilyInvite; locale: string }) {
  const t = await getTranslations("Family");
  const formatDate = (value: string) =>
    new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(new Date(value));

  return (
    <li className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div>
        <p className="text-sm">{t(`inviteStatus.${invite.status}`)}</p>
        <p className="text-sm text-muted">
          {invite.status === "pending"
            ? t("expires", { date: formatDate(invite.expiresAt) })
            : t("created", { date: formatDate(invite.createdAt) })}
        </p>
      </div>

      {invite.status === "pending" && (
        <form action={revokeInvite}>
          <input type="hidden" name="inviteId" value={invite.id} />
          <button
            type="submit"
            className="inline-flex min-h-11 items-center rounded-md border border-border px-3 text-sm hover:bg-border/60"
          >
            {t("cancel")}
          </button>
        </form>
      )}
    </li>
  );
}
