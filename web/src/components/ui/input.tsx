/**
 * The text/number/date input class string every form in this app has
 * repeated by hand since M1. Lifted out here, not redesigned — M3's item
 * form is the first one with enough conditional fields that copy-pasting
 * it an 8th time stopped being the cheaper option.
 */
export function Input({ className = "", ...props }: React.ComponentProps<"input">) {
  return (
    <input
      {...props}
      className={`mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm ${className}`}
    />
  );
}
