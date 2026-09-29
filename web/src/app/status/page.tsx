import { AppShell } from "@/components/app-shell";
import { getPing } from "@/lib/api/client";
import { requireMe } from "@/lib/api/session";

export const metadata = { title: "Stack status — mykomora" };

/**
 * The walking skeleton's screen, kept as a diagnostic.
 *
 * It makes the whole chain visible in a browser: this server component calls
 * core-api, which runs a generated query against Postgres. Both the healthy
 * and the unreachable path render as a normal screen.
 */
export default async function StatusPage() {
  const me = await requireMe("/status");
  const ping = await getPing();

  return (
    <AppShell me={me}>
      <h1 className="text-2xl font-semibold tracking-tight">Stack status</h1>
      <p className="mt-2 text-sm text-muted">
        What this page proves: the browser reached the web app, the web app reached core-api, and
        core-api reached Postgres.
      </p>

      <dl className="mt-8 divide-y divide-border rounded-lg border border-border">
        <Row label="web">
          <Status tone="ok">running</Status>
        </Row>

        <Row label="core-api">
          {ping.ok ? (
            <Status tone="ok">reachable</Status>
          ) : (
            <Status tone="degraded">unreachable</Status>
          )}
        </Row>

        <Row label="postgres">
          {ping.ok ? (
            <Status tone="ok">answering</Status>
          ) : (
            <Status tone="degraded">unknown</Status>
          )}
        </Row>

        <Row label="database clock">
          {ping.ok ? (
            <time dateTime={ping.data.now} className="font-mono text-sm">
              {ping.data.now}
            </time>
          ) : (
            <span className="text-sm text-muted">{ping.detail}</span>
          )}
        </Row>
      </dl>
    </AppShell>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-3">
      <dt className="text-sm text-muted">{label}</dt>
      <dd className="text-right">{children}</dd>
    </div>
  );
}

function Status({ tone, children }: { tone: "ok" | "degraded"; children: React.ReactNode }) {
  const toneClass = tone === "ok" ? "text-ok" : "text-degraded";

  return <span className={`text-sm font-medium ${toneClass}`}>{children}</span>;
}
