# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This repository is in the pre-scaffolding stage: it contains this file, a README, and the approved
specification. There is no code or build tooling yet, so there are no build/lint/test commands to
document — update this file once M0 (walking skeleton) lands and real commands exist.

The domain is settled and written down. Treat these as the source of truth and read them before
proposing domain changes:

- `docs/SPEC.md` — problem, concepts, data model, category/trait taxonomy, architecture, NFRs
- `docs/MILESTONES.md` — M0-M10 delivery plan with a "Done when" acceptance sentence each

Two constraints from the spec that are easy to violate and expensive to undo:

- **core-api owns identity.** The web app is a thin client. Auth must never migrate into Next.js,
  or the planned mobile client inherits a rewrite.
- **Every query is family-scoped.** Each sqlc query takes `family_id` and every handler resolves it
  from the session. An unscoped query is a review failure.

## Intended architecture

mykomora is planned as two services in this repo:

- **core-api** — Go REST API. Handlers/types are generated from an OpenAPI spec using `oapi-codegen`, so the spec is the source of truth for the HTTP contract — edit the OpenAPI YAML/JSON first, then regenerate, rather than hand-editing generated code. Dependency injection is wired with `uber-go/fx`.
- **web** — Next.js app written in TypeScript.

### Database

- Postgres.
- Data access in core-api uses `sqlc`: SQL is written by hand in `.sql` files and `sqlc` generates type-safe Go from it. There is no runtime ORM — raw SQL is the source of truth, and dropping to a hand-written query is expected to stay easy.
- Schema migrations use `goose` (plain `.sql` migration files, with Go-function migrations available for backfills that need logic beyond SQL).

### Internationalization

- Supported locales: `en` and `uk`.
- Default locale is chosen by request IP: `uk` for IP addresses geolocated to Ukraine, `en` otherwise.

## Code style

### Go (core-api)

- Linting: `golangci-lint`, strict/extensive set — govet, staticcheck, errcheck, unused, gofmt/goimports, revive, plus gosec (security), gocyclo (complexity limits), gocritic, dupl. The actual `.golangci.yml` gets written when core-api is scaffolded.
- Project layout: standard Go layout — `cmd/` for entrypoints, `internal/` for private application code, `pkg/` only for code meant to be imported externally.
- Error handling: wrap with `fmt.Errorf("...: %w", err)`, inspect with `errors.Is`/`errors.As`; use package-level sentinel errors (`var ErrNotFound = errors.New(...)`) for expected/handled cases.
- Imports: three groups via `goimports -local <module path>` — stdlib, third-party, then this module's own packages, blank-line separated.

### TypeScript / Next.js (web)

- Linting/formatting: `next/core-web-vitals` ESLint config + `@typescript-eslint` recommended rules, formatted with Prettier.
- Styling: Tailwind CSS, utility-first — no separate stylesheet files per component.
- Naming: kebab-case filenames (`user-profile-card.tsx`), PascalCase component names (`UserProfileCard`) — matches the App Router's kebab-case route folder convention.
- TypeScript: `strict: true` in tsconfig (plus `noUncheckedIndexedAccess`); imports use the `@/` path alias instead of long relative chains.

## Git conventions

- Branching model: trunk-based / GitHub flow — short-lived feature branches off `main`, merged via PR once reviewed and green. No long-lived `develop`/`release` branches.
- Branch naming: `type/short-description`, e.g. `feature/user-auth`, `fix/login-redirect`, `chore/upgrade-deps`.
- Commit messages: Conventional Commits — `feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:` with optional scope, e.g. `feat(core-api): add user endpoint`.
- PR merge strategy: squash merge — each PR lands as one commit on `main` with a clean message; full commit history lives on the PR only.
