import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Family = components["schemas"]["Family"];
export type FamilyMember = components["schemas"]["FamilyMember"];
export type FamilyInvite = components["schemas"]["FamilyInvite"];
export type CreatedInvite = components["schemas"]["CreatedInvite"];
export type InvitePreview = components["schemas"]["InvitePreview"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function getFamily() {
  return callApi<Ok<"getFamily", 200>>("/api/v1/family");
}

export function updateFamily(body: components["schemas"]["UpdateFamilyRequest"]) {
  return callApi<Ok<"updateFamily", 200>>("/api/v1/family", { method: "PATCH", body });
}

export function listFamilyMembers() {
  return callApi<Ok<"listFamilyMembers", 200>>("/api/v1/family/members");
}

export function removeFamilyMember(userId: string) {
  return callApi<void>(`/api/v1/family/members/${encodeURIComponent(userId)}`, {
    method: "DELETE",
  });
}

export function listFamilyInvites() {
  return callApi<Ok<"listFamilyInvites", 200>>("/api/v1/family/invites");
}

/**
 * Issues an invite. The link in the response is the only time it exists in
 * readable form — core-api stores a digest — so the UI has to show it there
 * and then.
 */
export function createFamilyInvite() {
  return callApi<Ok<"createFamilyInvite", 201>>("/api/v1/family/invites", { method: "POST" });
}

export function revokeFamilyInvite(inviteId: string) {
  return callApi<void>(`/api/v1/family/invites/${encodeURIComponent(inviteId)}`, {
    method: "DELETE",
  });
}

/** What an invite link leads to. Public: the invitee has no account yet. */
export function getInvitePreview(token: string) {
  return callApi<Ok<"getInvitePreview", 200>>(`/api/v1/invites/${encodeURIComponent(token)}`);
}
