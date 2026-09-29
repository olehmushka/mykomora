import { getPing } from "@/lib/api/client";

/**
 * The walking skeleton's only screen.
 *
 * It exists to make the whole chain visible in a browser: this server component
 * calls core-api, which runs a generated query against Postgres. Both the
 * healthy and the unreachable path render as a normal screen, which is the
 * product rule this app is built on — a missing answer is a state, not an error.
 */
export default async function Home() {
  const ping = await getPing();

  return (
    <main className="mx-auto w-full max-w-xl flex-1 px-4 py-10 sm:px-6 sm:py-16">
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">mykomora</h1>
      <p className="mt-2 text-sm text-muted">
        A family inventory for households spread across several places.
      </p>

      <section aria-labelledby="stack-status" className="mt-10">
        <h2 id="stack-status" className="text-sm font-medium uppercase tracking-wide text-muted">
          Stack status
        </h2>

        <dl className="mt-3 divide-y divide-border rounded-lg border border-border">
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

        {!ping.ok && (
          <p className="mt-3 text-sm text-muted">
            The API is not answering yet. With the local stack this usually means core-api is still
            starting — run <code className="font-mono">make dev</code> and reload.
          </p>
        )}
      </section>

      <p className="mt-10 text-xs text-muted">
        Milestone M0 — walking skeleton. Sign-in arrives in M1, Ukrainian in M2.
      </p>
    </main>
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
