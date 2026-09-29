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

M0 (walking skeleton) is done: the toolchain runs end to end and CI is green. Sign-in arrives in
M1, Ukrainian in M2. There is no domain code yet.

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

Both services live-reload from the mounted source, so editing Go or TypeScript is enough — no
rebuild. `make logs` follows the output, `make down` stops the stack, and `make help` lists every
target.

To work on the code outside Docker you also need Go 1.26 and Node 22. Go dev tools are pinned in
`core-api/go.mod` and need no installation.

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
| auth | Google OAuth2, tokens issued by core-api |
| deploy | Single VPS, Docker Compose, Caddy |

Mobile clients are planned later and will use the same core-api contract — which is why core-api,
not the web app, owns identity.
