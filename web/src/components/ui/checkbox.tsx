/** A checkbox with its own label, touch-target sized like every other control. */
export function Checkbox({
  label,
  className = "",
  ...props
}: React.ComponentProps<"input"> & { label: React.ReactNode }) {
  return (
    <label className={`inline-flex min-h-11 items-center gap-2 text-sm ${className}`}>
      <input {...props} type="checkbox" className="size-4" />
      {label}
    </label>
  );
}
