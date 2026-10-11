/** Select's half of input.tsx's split — same classes, same reasoning. */
export function Select({ className = "", ...props }: React.ComponentProps<"select">) {
  return (
    <select
      {...props}
      className={`mt-1 min-h-11 w-full rounded-md border border-border bg-transparent px-3 text-sm ${className}`}
    />
  );
}
