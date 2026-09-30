"use client";

import { useActionState } from "react";

import { renameFamily, type ActionState } from "@/app/settings/family/actions";

export function RenameForm({ currentName }: { currentName: string }) {
  const [state, formAction, pending] = useActionState<ActionState, FormData>(renameFamily, {});

  return (
    <form action={formAction} className="mt-3">
      <label htmlFor="family-name" className="sr-only">
        Family name
      </label>

      <div className="flex flex-wrap items-center gap-2">
        <input
          id="family-name"
          name="name"
          defaultValue={currentName}
          maxLength={120}
          required
          className="min-h-11 min-w-0 flex-1 rounded-md border border-border bg-transparent px-3 text-sm"
        />
        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? "Saving…" : "Save"}
        </button>
      </div>

      {state.error && (
        <p role="alert" className="mt-2 text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}
