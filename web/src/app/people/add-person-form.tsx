"use client";

import { useActionState, useRef } from "react";

import { addPerson, type ActionState } from "@/app/people/actions";

/**
 * Adding a person, kept to three fields.
 *
 * Entry burden is what kills home inventories (SPEC 1), and that starts here:
 * a name is the only thing required, because a half-filled person is a normal
 * record and the rest can be added when it is actually known.
 */
export function AddPersonForm() {
  const form = useRef<HTMLFormElement>(null);

  const [state, formAction, pending] = useActionState<ActionState, FormData>(
    async (previous, data) => {
      const result = await addPerson(previous, data);
      if (!result.error) {
        form.current?.reset();
      }

      return result;
    },
    {},
  );

  return (
    <form ref={form} action={formAction} className="mt-3 rounded-lg border border-border px-4 py-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label htmlFor="person-name" className="text-sm font-medium">
            Name
          </label>
          <input
            id="person-name"
            name="name"
            required
            maxLength={120}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>

        <div>
          <label htmlFor="person-birthdate" className="text-sm font-medium">
            Birthdate <span className="font-normal text-muted">(optional)</span>
          </label>
          <input
            id="person-birthdate"
            name="birthdate"
            type="date"
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm"
          />
        </div>
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <label className="inline-flex min-h-11 items-center gap-2 text-sm">
          <input name="isChild" type="checkbox" className="size-4" />A child
        </label>

        <button
          type="submit"
          disabled={pending}
          className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-border/60 disabled:opacity-60"
        >
          {pending ? "Adding…" : "Add person"}
        </button>
      </div>

      {state.error && (
        <p role="alert" className="mt-3 text-sm text-degraded">
          {state.error}
        </p>
      )}
    </form>
  );
}
