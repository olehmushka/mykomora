/**
 * A neutral block of explanation.
 *
 * Deliberately not styled as an error by default. A family with no people
 * recorded yet, or an invite that has already been used, are ordinary states
 * of this application — rendering them in alarm colours would be lying about
 * how well things are going (SPEC 1: "every screen must be useful with
 * incomplete data").
 */
export function Notice({
  tone = "neutral",
  title,
  children,
}: {
  tone?: "neutral" | "warning";
  title?: string;
  children?: React.ReactNode;
}) {
  const border = tone === "warning" ? "border-degraded/50" : "border-border";

  return (
    <div
      className={`rounded-lg border ${border} px-4 py-3`}
      role={tone === "warning" ? "alert" : undefined}
    >
      {title && <p className="text-sm font-medium">{title}</p>}
      {children && <div className="mt-1 text-sm text-muted">{children}</div>}
    </div>
  );
}
