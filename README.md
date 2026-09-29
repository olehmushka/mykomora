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

Pre-scaffolding. The specification is approved; implementation starts at M0.

- [docs/SPEC.md](docs/SPEC.md) — problem, concepts, data model, taxonomy, architecture
- [docs/MILESTONES.md](docs/MILESTONES.md) — the M0–M10 delivery plan

## Planned stack

| Part | Choice |
|---|---|
| core-api | Go, OpenAPI-first (`oapi-codegen`), `uber-go/fx`, `sqlc`, `goose` |
| web | Next.js, TypeScript strict, Tailwind, `next-intl` |
| data | Postgres, S3-compatible object storage |
| auth | Google OAuth2, tokens issued by core-api |
| deploy | Single VPS, Docker Compose, Caddy |

Mobile clients are planned later and will use the same core-api contract — which is why core-api,
not the web app, owns identity.
