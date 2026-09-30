# mykomora

A family inventory for households spread across several places.

*Komora* (комора) is Ukrainian for a pantry or storeroom. mykomora is the family's storeroom, made
searchable: every item is a real record with category-appropriate traits, lives in a known place —
optionally inside a box that can be moved as a unit — and is shared with the whole family.

It exists to answer two questions without opening a single box:

- **What do we already own?** (so it is not bought twice)
- **What fits the kids now, and what fits next?** (and which box is it in)

Bilingual from the start: Ukrainian and English, with the default chosen by request IP.

## Status

M1 (identity and family) is done: two Google accounts can share one family, and every query is
scoped to it. Ukrainian arrives in M2, along with places and the category taxonomy. There are no
items yet — those are M3.

- [docs/SPEC.md](docs/SPEC.md) — problem, concepts, data model, taxonomy, architecture
- [docs/MILESTONES.md](docs/MILESTONES.md) — the M0–M10 delivery plan

## Getting started

You need Docker with Compose. Everything else runs inside containers.

```sh
make dev
```

That builds and starts the stack, applies migrations, and prints the URLs:

| | |
|---|---|
| App | <http://localhost:8080> |
| Liveness | <http://localhost:8080/healthz> |
| Round-trip probe | <http://localhost:8080/api/v1/ping> |

Sign-in needs a Google OAuth client; see below. Without one the stack still runs — the sign-in
page renders and says so.

Both services live-reload from the mounted source, so editing Go or TypeScript is enough — no
rebuild. `make logs` follows the output, `make down` stops the stack, and `make help` lists every
target.

To work on the code outside Docker you also need Go 1.26 and Node 22. Go dev tools are pinned in
`core-api/go.mod` and need no installation.

### Google sign-in

core-api performs the OAuth2 exchange itself and issues its own tokens, so the only thing you
need locally is a client:

1. In the [Google Cloud Console](https://console.cloud.google.com/apis/credentials), create an
   **OAuth client ID** of type *Web application*.
2. Add `http://localhost:8080/api/v1/auth/google/callback` as an **authorised redirect URI**. It
   must match exactly, and it must point at Caddy on port 8080 — core-api's own port 8081 is a
   different origin, so the session cookies would not apply to the app.
3. Put the client id and secret in `deploy/.env` (`make dev` creates it from
   `deploy/.env.example` on first run):

   ```sh
   GOOGLE_CLIENT_ID=...apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=...
   ```

4. `make down && make dev`.

There is no offline or developer bypass: a credential-free sign-in path gated only by an
environment variable is the kind of thing that survives into production, so it does not exist in
the binary at all. Tests do not need any of this — they run against a local OIDC issuer.

To try the whole milestone you need two Google accounts, or one account and a second browser
profile: sign in, generate an invite under **Family settings**, and open the link as the other
account.

### Before you push

```sh
make check
```

Lint, type-check and unit tests for both services — the same things CI runs, minus the jobs that
need Docker. `make test-integration` adds the testcontainers tests.

## Planned stack

| Part | Choice |
|---|---|
| core-api | Go, OpenAPI-first (`oapi-codegen`), `uber-go/fx`, `sqlc`, `goose` |
| web | Next.js, TypeScript strict, Tailwind, `next-intl` |
| data | Postgres 18, S3-compatible object storage |
| auth | Google OAuth2 (PKCE), access JWT + rotating refresh tokens, issued by core-api |
| deploy | Single VPS, Docker Compose, Caddy |

Mobile clients are planned later and will use the same core-api contract — which is why core-api,
not the web app, owns identity.
