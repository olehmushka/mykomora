# mykomora — Milestone Plan

Companion to [SPEC.md](./SPEC.md). Every milestone is independently shippable and leaves the app in a usable state.

Estimates assume one part-time developer and are **relative sizes, not commitments**.

| # | Milestone | Size | Ships |
|---|---|---|---|
| M0 | Walking skeleton | 3–4 d | A running local stack and green CI |
| M1 | Identity & family | 5–6 d | Two Google accounts sharing one family |
| M2 | Places, taxonomy & i18n | 5–6 d | The app in Ukrainian and English |
| M3 | Items core | 8–10 d | Real items, in real boxes, with history |
| M4 | Photos & attachments | 4–5 d | Photograph an item from a phone |
| M5 | Search, filter & browse | 5–6 d | Find anything in under a second |
| M6 | Fast entry | 6–8 d | 20 items into a box in 10 minutes |
| M7 | Consumables & alerts | 4–5 d | What needs attention today |
| M8 | Kids' sizes & seasons | 4–5 d | What fits now, what fits next |
| M9 | Lending & timeline | 3 d | Who took it and when it returns |
| M10 | Hardening & go-live | 4–5 d | Running in production, backed up |

---

## M0 — Walking skeleton *(3–4 days)*

Prove the entire toolchain end to end before writing any domain code.

- Repo layout: `core-api/`, `web/`, `deploy/`, `docs/`
- core-api: Go module, `fx` wiring, chi router, `/healthz`, `.golangci.yml`, `Makefile`
- OpenAPI 3.1 file with one trivial endpoint → `oapi-codegen` → handler → `sqlc` query → Postgres
- goose migration #1: extensions `pgcrypto`, `unaccent`, `pg_trgm`
- web: Next.js, TS `strict` + `noUncheckedIndexedAccess`, Tailwind, ESLint/Prettier, `@/` alias, calling that endpoint
- `deploy/docker-compose.yml` — postgres, core-api, web, caddy, LocalStack (local S3) — plus `.env.example`
- GitHub Actions: lint, test and build both services; codegen-drift check; migration up/down check

**Done when:** `make dev` gives a working local stack and CI is green on a PR to `main`.

> **Delivered with one substitution.** MinIO was the planned local S3 stand-in, but its images were
> withdrawn from Docker Hub — `minio/minio` and `minio/mc` no longer resolve — so a fresh clone
> could not start the stack. LocalStack takes its place, pinned to the 4.x line because `latest` is
> now a licensed build. The S3 API contract is unchanged and production remains R2/B2.

---

## M1 — Identity & family *(5–6 days)*

- Google OAuth2 (authorization code + PKCE) handled **in core-api**; ID token verified against Google's JWKS
- Access JWT (15 min) + rotating refresh tokens with reuse detection
- Endpoints: `/auth/google/start`, `/auth/google/callback`, `/auth/refresh`, `/auth/logout`, `/me`
- Tables: `users`, `families`, `family_members`, `family_invites`, `refresh_tokens`, `people`
- First sign-in creates a family and makes the user its owner
- Invite flow: owner generates a single-use link (token hashed, 7-day TTL); the invitee signs in with Google and joins. No email — the link is shared over any channel the family already uses.
- Family-scope middleware, plus a sqlc convention that makes an unscoped query obvious in review
- Web: sign-in, session handling, family settings, member list, invite UI, people (children) management

**Done when:** two Google accounts share one family and both see the same (empty) inventory.

---

## M2 — Places, taxonomy & i18n *(5–6 days)*

- `locations` and `rooms` CRUD with archive
- `categories` + `trait_defs`, seeded by a goose **Go** migration with all 23 built-in categories and their traits
- Family-defined categories and custom fields (same tables, `family_id` set)
- `GET /taxonomy` returning the localised category tree and trait definitions, ETag-cached
- Go message catalogue for API strings; `next-intl` with `uk`/`en` bundles; locale middleware with GeoIP (`UA` → `uk`) and user-preference override
- Size-normalisation lookup table seeded
- Web: locale switcher, locations/rooms management, category browser, custom-field editor

**Done when:** the whole UI renders correctly in Ukrainian and English, and a Ukrainian IP lands on `uk` by default.

---

## M3 — Items core *(8–10 days — the largest milestone)*

- `items` with hybrid columns + `traits` JSONB (GIN index), `item_events`, `tags` / `item_tags`
- Trait validation service: values checked against `trait_defs` for type, enum membership and requiredness; unknown keys rejected
- Containers: `parent_item_id`, cycle and depth guards, effective-place resolution, move-container-with-contents
- Lifecycle states with legal-transition rules; archive rather than delete
- Audit events written **in the same transaction** as every mutation
- REST: `POST/GET/PATCH/DELETE /items`, `POST /items/{id}/move`, `POST /items/{id}/status`, `GET /items/{id}/contents`, `GET /items/{id}/events`
- Web: item create/edit form rendered from the trait schema, item detail page, container view, location/room browse, mobile-first layout

**Done when:** a real box of kids' clothes is in the system, can be moved as a unit, and its history is visible.

---

## M4 — Photos & attachments *(4–5 days)*

- `item_photos`, `item_attachments`; storage interface with an S3-compatible implementation (LocalStack locally, R2/B2 in production)
- `POST /uploads/presign` plus confirm endpoints; server-side content-type and size validation; private bucket, presigned GETs
- Client-side resize and thumbnail generation before upload; camera capture on mobile
- Photo gallery with primary selection and reordering; receipts and manuals on the item detail page
- Orphaned-object cleanup job

**Done when:** photographing and attaching an item from a phone is one flow, with no desktop involved.

---

## M5 — Search, filter & browse *(5–6 days)*

- `search_tsv` generated column with GIN, `pg_trgm` index on `name`, `unaccent`
- `GET /items` filters: text query, category (including descendants), location/room, person, tag, status, container, trait filters, expiry and warranty windows, price range; keyset pagination; stable sorts
- Duplicate hints on create, via trigram similarity within the family
- Dashboard: counts per location and category, total purchase value, recent activity
- Web: debounced search bar, filter panel driven by `trait_defs.filterable`, list/grid results, and empty or partial states that never shame the user for incomplete data

**Done when:** *«де зимові чоботи 32 розміру»* finds the item and names the box and the room, in under a second.

---

## M6 — Fast entry *(6–8 days — load-bearing, not optional)*

This is the milestone that decides whether the app survives contact with reality.

- Batch add: shared fields plus variable rows, creating N items in one transaction
- Duplicate-item action
- Barcode/ISBN scanning in the browser (`BarcodeDetector`, ZXing-wasm fallback)
- `GET /lookup/barcode/{code}` proxying Open Food Facts, Open Beauty Facts, Open Products Facts and Google Books, cached in `product_cache`, with a fast and graceful not-found path
- CSV import: upload → column mapping → dry-run validation report → commit, with per-row errors
- CSV export of the full inventory

**Done when:** 20 items of kids' clothing go into a box in under 10 minutes, and a shelf of books goes in by scanning.

---

## M7 — Consumables & the Attention panel *(4–5 days)*

- Quantity, unit and minimum quantity with quick +/− adjustment, emitting a `quantity_changed` event
- Expiry; `opened_at` + `pao_months` producing a computed effective expiry
- Alert engine — query-based, no cron needed at this scale: expired, expiring within 30 days, below minimum, warranty ending within 60 days, loan overdue
- `alert_dismissals` (snooze until a date); `GET /alerts`
- Web: Attention panel on the dashboard with badge counts; shopping list = everything below minimum, exportable as text

**Done when:** opening the app shows a truthful, dismissible list of what needs action today.

---

## M8 — Kids' sizes & seasons *(4–5 days)*

- `person_sizes` with history; normalisation applied on item write
- `GET /people/{id}/fits?offset=0|1` → "fits now" and "next size up", grouped by effective place
- Outgrown flow: bulk status change to `to_give_away`
- Seasonal pack/unpack: select a container, flip its contents between `stored` and `in_use`, one event per item, one undo
- Web: child profile, fits-now and next-size views, bulk selection UI

**Done when:** before buying anything for a child, one screen shows what already fits, what fits next, and where it physically is.

---

## M9 — Lending & timeline *(3 days)*

- `loans`: lend to a person or a free-text outsider, with a due date and a return action; integrated with item status; overdue feeds the Attention panel
- Item timeline over `item_events`, with actor avatars and human-readable, localised descriptions
- "Currently lent out" view

**Done when:** "who took the drill and when is it coming back" is one tap away.

---

## M10 — Hardening & go-live *(4–5 days)*

- Nightly `pg_dump` and object-storage sync to a backup bucket, with a retention policy and **a restore rehearsal documented in `docs/runbook.md`**
- Rate limits (auth, lookup, upload), CSRF, security headers and CSP, dependency scanning
- Structured logging with request IDs, `/readyz`, basic metrics, uptime monitoring
- Family data export (JSON + CSV + photos) and a hard-delete path
- Production deploy on the VPS: Caddy with automatic TLS, Compose file, secrets handling, deploy runbook
- Playwright smoke tests over the critical path: sign in → add item → find item → move box

**Done when:** the family is using it on real data, and losing the VPS would not lose the inventory.

---

## Sequencing notes

- **M0 → M1 → M2 → M3 is a hard chain.** M4, M5 and M6 can be reordered.
- **Dogfood from the end of M3.** Enter one real room and see what hurts before building M5–M8.
- **If entry fatigue appears during M3 dogfooding, pull M6 forward** ahead of photos and search. Entry speed is the survival constraint.
- **M2's taxonomy seed is the most reversible-looking decision that is actually expensive.** Traits are versioned by migration, so spend the time to get the clothing and cosmetics trait sets right — they carry the two primary jobs.

---

## Post-v1 backlog

PWA with offline reads · mobile apps on the same API · AI photo recognition · printable QR labels for boxes · email digests · per-item privacy · multi-currency · insurance report export · borrow requests between families.

---

## Verification, per milestone

- `make lint test` — `golangci-lint` (govet, staticcheck, errcheck, unused, gofmt/goimports, revive, gosec, gocyclo, gocritic, dupl) and `go test ./...`
- Go integration tests run sqlc queries against a real Postgres via `testcontainers-go`; handler tests via `httptest` against the generated strict interface
- `sqlc vet`, plus a codegen drift check: regenerate from `openapi.yaml` and `sqlc.yaml`, then `git diff --exit-code`
- `goose up` followed by `goose down` to base on a scratch database, for every migration, in CI
- web: `tsc --noEmit`, `next lint`, `vitest`; from M10, `playwright test`
- Manual end-to-end on `make dev`, exercising the milestone's **Done when** sentence on a phone browser, in both `uk` and `en`
