/** A labeled field, the `<div className="flex-1">...</div>` wrapper every form repeats. */
export function FieldRow({
  label,
  htmlFor,
  hint,
  className = "",
  children,
}: {
  label: React.ReactNode;
  htmlFor: string;
  hint?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={`flex-1 ${className}`}>
      <label htmlFor={htmlFor} className="text-sm font-medium">
        {label}
        {hint && <span className="font-normal text-muted"> {hint}</span>}
      </label>
      {children}
    </div>
  );
}
