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

> **Delivered, with three notes.**
>
> *The web app's session refresh lives in `web/src/proxy.ts`, not `middleware.ts`.* Next 16
> deprecated and renamed the middleware convention; the mechanism is unchanged, and it is still
> the only place that can rotate a cookie before a page renders.
>
> *`families` needed a stronger reading of the tenancy invariant, not an exemption.* It is the one
> tenant table whose own id is the tenant key, so there is no `family_id` column to filter on.
> Rather than allowlisting the two queries that read and write a family row, both now scope
> themselves through `family_members` — the caller proves membership instead of asserting it, and
> the vet rule sees a real scope. The allowlist grew only by the three `users` queries, which run
> before any tenant exists, and by the reuse-detection revoke, which crosses families on purpose.
>
> *The browser-facing auth endpoints answer in redirects, not status codes.* A deployment with no
> Google client is a supported state, and the person in front of the browser is owed a sign-in
> page that explains itself rather than a JSON body rendered raw.

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

> **Delivered, with four notes.**
>
> *GeoIP was dropped entirely.* SPEC's locale order includes an IP geolocated against MaxMind's
> GeoLite2-Country database; that step would mean an account, a license key and a
> redistribution/update story for a database file, for a single-family app with no public sign-up.
> Locale resolves from the `NEXT_LOCALE` cookie (web), then `Accept-Language`, then English — no
> client IP anywhere in the chain. This milestone's own "Done when" sentence no longer holds for
> its second clause: a Ukrainian IP with no cookie and an English-language browser now sees `en` on
> first load, not `uk`.
>
> *Migrations now run through `core-api/cmd/migrate`, not the stock `goose` CLI.* The 23-category
> seed is this repo's first Go-function goose migration
> (`db/migrations/00006_taxonomy_seed.go`), and `goose.AddMigrationContext` only registers a
> migration with the process that imports the package defining it — which the stock CLI never
> does. `make migrate-up/down/reset/status` and the `migrate` Compose service now go through that
> command instead; `make migrate-create` (scaffolding a new filename) is unaffected, and
> `make migrate-create-go` was added alongside it.
>
> *Locale resolution splits across the two services rather than living in one chain.* core-api's
> `internal/locale.Resolver` only ever sees the stored preference (`users.locale`) and the
> `Accept-Language` it was sent; the web app resolves the `NEXT_LOCALE` cookie itself before a
> request ever reaches core-api, and sends the result as that header. This is also correct for a
> future mobile client, which has no concept of that cookie at all.
>
> *M1's existing error strings were not retrofitted into the Go message catalogue.* The catalogue
> (`internal/catalogue`) covers the 23 categories' and their traits' labels plus this milestone's
> own new validation messages; `people` and `family`'s hardcoded strings are migrated
> opportunistically as those files are next touched, not as a blanket rewrite — the web app doesn't
> render them bilingually today either.

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

> **Delivered, with five notes.**
>
> *The hot-column ↔ trait-key mapping was already decided for us.* M2's seed migration
> (`db/migrations/taxonomy_seed_data.go`) placeholder-seeded `size`, `size_system`, `season`,
> `for_person`, `expires_at`, `opened_at`, `pao_months`, `quantity`, `unit`, `min_quantity`,
> `serial_number` and `warranty_until` as `data_type = 'text'` specifically so this milestone
> could give them a real column and, where the vocabulary is closed, a real `data_type`. This
> migration's own `UPDATE trait_defs` statements do exactly that for `season`/`condition`/
> `size_system` (→ `enum`), `pao_months` (→ `integer`), `quantity`/`min_quantity` (→ `number`) and
> the three date fields (→ `date`). `for_person` and `size` stay `text` placeholders on purpose:
> `for_person`'s valid values are a family's dynamic `people` list, and `size`'s validation
> against `size_lookup` needs a `size_type` column `trait_defs` doesn't have — both are M8's job.
> A request's `traits` object may never carry one of these keys; the validator rejects it with a
> 400 rather than silently routing it, so there is exactly one place this rule lives
> (`internal/items/traits.go`).
>
> *`item_condition` and `item_season` needed vocabularies the spec never named.* Locked in as
> `(new, like_new, good, worn, poor)` and `(summer, winter, demi, all_season)` — a product call,
> not an engineering one, made explicitly rather than guessed past.
>
> *Containment is read-resolved, never denormalised.* A nested item's own `location_id`/
> `room_id`/`spot` are set once at creation (so the `NOT NULL` column has something to hold) and
> become cosmetic the moment it has a parent; effective place is always the root ancestor's place,
> walked via one recursive CTE (`GetItemAncestorChain`) at read time. Moving a container therefore
> writes exactly one row and one event, and every descendant's effective place changes for free.
> `GET /items/{id}/contents` returns direct children only — drilling into a nested container is
> one link at a time, not a flattened subtree — and location/room browse (`GET /items`) shows root
> items only, for the same reason.
>
> *Tags got full management now, not deferred to M5.* Beyond the milestone's own `tags`/
> `item_tags` tables, this PR also ships a standalone `/settings/tags` page (create/rename/delete)
> and dedicated `POST /items/{id}/tags` + `DELETE /items/{id}/tags/{tagId}` endpoints, pulled
> forward from M5's "browse by tag" on request, rather than the minimal replace-all-on-the-item-
> form shape originally scoped.
>
> *`CreateItem`/`MoveItem` validate referenced rows with their own Go-level reads rather than
> folding the checks into the `INSERT`'s `WHERE EXISTS` the way `rooms.sql`/`trait_defs.sql` do.*
> Those tables have one referential check each; items has up to four (category, location, room,
> parent, person) plus trait, depth and cycle validation that cannot be expressed as a single SQL
> guard anyway. The small TOCTOU window this opens cannot leak across families — every write still
> carries its own `family_id` — so the trade is a plainer `INSERT`, not a safety gap.

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

> **Delivered, with five notes.**
>
> *Matching is "most of the words", not "all of them".* The milestone's own query, «де зимові
> чоботи 32 розміру», contains words no item says ("де", "розміру"). Stopwords ("де", "розмір",
> "size"…) are dropped, and an item must contain at least ⌈0.6·n⌉ of the remaining words, ranked by
> how many it has plus trigram similarity of the name. Postgres has no Ukrainian stemmer, so words
> are cut by hand before prefix matching (six letters or more lose two, four or five lose one):
> "чоботи" becomes `чобо:*`. Numbers stay exact so "32" does not match "320".
>
> *Season and condition are searchable by word.* They are stored as English enum labels, yet people
> type «зимові». `item_enum_keywords()` (migration 00010) maps each label to its Ukrainian and
> English words inside the generated `search_tsv`; a new enum label has to be taught to it. The
> document also contains `traits::text`, so JSON keys are searchable too — noise, but the only
> option inside a generated column.
>
> *`GET /items` is the single list-and-search endpoint, and `offset` is gone.* Pagination is keyset
> on `(sort_key, id)` with an opaque cursor that is rejected if replayed against a different sort.
> A bare browse or a place-only filter still lists root items; any words or other filter searches
> nested items too, and each hit carries its container path and room so the UI can say "Box ›
> Bedroom · Flat". Location and room filters match the *effective* place (the root container's).
> Trait filters are one JSON `traits` parameter, equality only, validated against the chosen
> category's `filterable` fields; numeric ranges on traits are not built yet.
>
> *pgx needed a text codec for `tsvector`.* `SELECT *` and `RETURNING *` read the new column and pgx
> has no binary codec for it. `postgres.ConfigurePool` registers one, and sqlc maps the column to
> `string`; nothing in Go reads it.
>
> *Dashboard counts nested items under their root's location; value is per currency.* No
> conversion between currencies (post-v1), and only items with a price that are still owned count.
> Duplicate hints use trigram similarity ≥ 0.4 on the unaccented name, top five, advisory only.
> Results paginate with a "Next page" link rather than infinite scroll. Search over 20k seeded
> items measured ≈33 ms warm in the integration suite.

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

> **Delivered, with five notes.**
>
> *A batch is N complete create requests, not "shared fields + rows" on the wire.* `POST /items/batch`
> takes up to 100 full `CreateItemRequest`s; the web screen merges the shared header into each row.
> Every row goes through the same `prepareCreate` as a single create, so batch, duplicate and import
> cannot validate differently, and one transaction writes all rows plus one `created` event each. A
> failure names the row ("row 3: …") and creates nothing. Duplicate copies fields, traits and tags but
> not photos, serial number, events or contents.
>
> *Barcode lookup asks every catalogue at once and caches misses.* Open Food/Beauty/Products Facts and,
> for ISBNs, Google Books are queried concurrently under a 2.5 s budget; the first hit wins. Hits are
> cached 30 days and *confirmed* misses one day in `product_cache` (a global table, so its two queries
> are on the `family-scoped` allowlist); a catalogue outage is never cached as a miss. Not-found is a
> 200 with `found: false`. `GOOGLE_BOOKS_API_KEY` is optional. The ZXing-wasm fallback fetches its
> `.wasm` from a CDN by default — pin it locally before the M10 CSP work.
>
> *CSV export is a stream; import is upload → inspect → dry run → commit.* Export pages by id through
> a pipe, writes places, container, person and tags by name and the category by slug, and quotes cells
> starting `= + - @` with an apostrophe (undone on import). Import uploads to
> `families/{id}/imports/…` by presigned PUT, so core-api reads it back (`storage.Client.GetObject`,
> capped at 5 MiB / 2000 rows); the existing sweeper removes an abandoned file after 24 h and commit
> deletes it. Delimiter (comma, semicolon, tab), BOM and Ukrainian headings are handled.
>
> *Import limits worth knowing.* Commit is all-or-nothing and refuses a file with any bad row. Tags,
> people, locations and rooms must already exist (matched by name, case-insensitively); a `container`
> cell must name exactly one existing container, so boxes created by the same file are not visible
> to it. Categories are matched by slug.
>
> *Not built.* Scanning is wired into the item form and the batch screen only, and a scanned code is
> not stored on the item (there is no barcode column yet).

---

## M7 — Consumables & the Attention panel *(4–5 days)*

- Quantity, unit and minimum quantity with quick +/− adjustment, emitting a `quantity_changed` event
- Expiry; `opened_at` + `pao_months` producing a computed effective expiry
- Alert engine — query-based, no cron needed at this scale: expired, expiring within 30 days, below minimum, warranty ending within 60 days, loan overdue
- `alert_dismissals` (snooze until a date); `GET /alerts`
- Web: Attention panel on the dashboard with badge counts; shopping list = everything below minimum, exportable as text

**Done when:** opening the app shows a truthful, dismissible list of what needs action today.

> **Delivered, with four notes.**
>
> *Alerts are computed, never stored.* `GET /alerts` classifies the family's live items on every call
> (`internal/items/alerts.go`); only snoozes persist, in `alert_dismissals`, one row per
> (family, item, kind). A snooze is per family, offered as 1 day / 1 week / 1 month, and undoable. It
> expires by date only: it does not lift early if the item's own dates change. "Today" is the UTC
> calendar day.
>
> *Effective expiry is the earlier of the printed date and `opened_at + pao_months`.* The same rule
> exists twice — in `alerts.sql` for filtering and in `apimap.EffectiveExpiry` for `Item.effectiveExpiresAt`
> — and both clamp to month end like Postgres (31 Jan + 1 month = 28 Feb). Windows: expired is before
> today, expiring is today through +30 days, warranty is today through +60 (an already-ended warranty
> is not an alert). Below minimum is strictly `quantity < min_quantity`.
>
> *Quantity changes are one statement.* `POST /items/{id}/quantity` takes a `delta` or an absolute
> `quantity`, locks the row, clamps at zero and writes `quantity_changed {from, to}` in the same
> transaction; a change that changes nothing writes no event.
>
> *Deferred.* `loan_overdue` waits for the `loans` table in M9 (`AlertKind` is an enum, so M9 adds a
> value). Adding the `expired` value to it made oapi-codegen prefix the other enums' colliding
> constants — `api.Pending` is now `api.InviteStatusPending`. The shopping list is below-minimum only;
> the plain-text export is built in the browser (copy or `.txt` download).

---

## M8 — Kids' sizes & seasons *(4–5 days)*

- `person_sizes` with history; normalisation applied on item write
- `GET /people/{id}/fits?offset=0|1` → "fits now" and "next size up", grouped by effective place
- Outgrown flow: bulk status change to `to_give_away`
- Seasonal pack/unpack: select a container, flip its contents between `stored` and `in_use`, one event per item, one undo
- Web: child profile, fits-now and next-size views, bulk selection UI

**Done when:** before buying anything for a child, one screen shows what already fits, what fits next, and where it physically is.

> **Delivered, with five notes.**
>
> *Normalisation is a database rule.* Migration 00013 adds `size_norm_range()` and a trigger on `items`
> (and on `person_sizes`), so create, update, batch add and CSV import all normalise identically and the
> migration backfilled existing items. Besides an exact `size_lookup` hit it resolves ranges (`104–110`)
> and, for `kids_eu`, ignores an age suffix (`5T`, `6y`). An unmapped label stores with a NULL range.
>
> *Fits are strict overlaps.* `GET /people/{id}/fits?offset=0|1` returns one set per size type, each item
> with its effective place (the same `ItemHit` as search). An item fits when its range overlaps the
> person's strictly, so touching at a bracket edge is not a fit. "Next size up" is the range of the same
> width directly above the current one. Items earmarked for someone else, and anything not `stored` or
> `in_use`, are left out. An item is shoe-sized when its system is `shoe_*`, else clothing.
>
> *Head sizes are recorded but not matched*: an item carries no head size, so there is nothing to match.
>
> *One batch id, one undo.* `item_events.batch_id` groups the events of `POST /items/bulk-status` (active
> statuses only) and `POST /items/{id}/season-flip` (direct contents only: `in_use` ⇄ `stored`).
> `POST /item-batches/{id}/undo` restores each item still at the status the batch gave it and counts the
> rest as skipped.
>
> *Deferred.* The browser check of the new screens (child profile, fits, pack/unpack) is still owed.

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
