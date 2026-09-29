# mykomora — Product Specification

> *Komora* (комора) — a pantry or storeroom. mykomora is the family's storeroom, made searchable.

Status: **approved spec, pre-implementation.** Milestones live in [MILESTONES.md](./MILESTONES.md).

---

## 1. Problem

A family whose belongings are spread across several places in one country — the flat, the parents' place, the garage, a storage room — has no reliable answer to *"what do we own, and where is it?"*

The cost is concrete and recurring:

- buying something that already exists in a box two locations away;
- unpacking three boxes to find one winter jacket;
- kids' clothes outgrown before they were ever worn;
- cosmetics and medicine discovered expired.

### Why build instead of buy

| Tool | Strength | Gap for this use case |
|---|---|---|
| Sortly | QR/barcode, custom fields, polished | Business pricing, generic custom fields with no category semantics |
| HomeZada | Inventory + maintenance + documents | Single-property centric, heavy, no Ukrainian |
| Homebox (OSS, Go) | Self-hosted, locations, labels, warranties, CSV | Flat generic item model, weak family roles, no consumables, no Ukrainian |
| Grocy | Best-in-class consumables: stock, expiry, min-qty | Pantry only — no clothes, toys or books |
| Itemlist / Vorby / SaveOr | Multi-location and family sharing as paid tiers | Closed SaaS, no Ukrainian, no API we own |
| Outgrow / Hand Me Down | Kids' sizes, hand-me-down reservations, tag OCR | Single-purpose silo — yet another app |

The unoccupied combination is: **multi-location family + genuine per-category traits + uk/en + an API we own** (so mobile clients are a client of our own contract, not a vendor's).

### The dominant risk

Home inventories die of **data-entry burden**, not missing features. Public evidence is consistent: cataloguing a full home is on the order of 30–50 hours of typing, free-tier churn runs around a third per year, and a half-updated inventory is worse than none because trust collapses and people stop consulting it.

This project deliberately chooses **item-level granularity** — the highest-value and highest-cost option. Two consequences are binding on every design decision that follows:

1. **Entry speed is a feature, not an optimisation.** Batch entry, duplication, barcode lookup and CSV import are scheduled as a first-class milestone (M6), not a backlog item.
2. **Every screen must be useful with incomplete data.** A partially-filled item, a box with unknown contents, and a category with no items are all normal states and must never be rendered as errors or nagging.

### Intended outcome

A family-shared, bilingual catalogue where every item is a real record with category-appropriate traits, lives in a known place (optionally inside a box that can be moved as a unit), and which can answer *"what size 110 winter clothes do we have, and which box are they in?"* without anyone opening a box.

---

## 2. Scope

### Primary jobs, in priority order

1. **Know what we own** — avoid re-buying; see what exists before shopping, packing or moving.
2. **Kids: what fits now, what's next** — clothes and toys by size and season, across locations.
3. **Consumables, lightly** — quantity, running low, expiry.
4. **Find a specific thing** — search that leads to a room, a spot, or a box.

### Product decisions

| Area | Decision |
|---|---|
| Granularity | Every item individually |
| Devices | Mobile-web first (phone in hand at the wardrobe); desktop for bulk work |
| Audience | One family in practice; modelled multi-tenant from day one |
| Locations | Several places, one country, one currency |
| Place model | Location → Room → free-text spot |
| Containers | An item may contain items; moving it moves its contents |
| Traits | Built-in per-category schemas **plus** family-defined custom fields |
| Auth | Google OAuth2 only |
| Fast entry | Batch/duplicate add, barcode & ISBN lookup, CSV import/export |
| Money & docs | Purchase date + price, warranty expiry, receipts/manuals as attachments |
| Notifications | **In-app only** — no outbound email anywhere in the system |
| Movement | Lifecycle states, lending, full audit trail |
| Hosting | Single VPS, Docker Compose, S3-compatible object storage |

### Non-goals for v1

Mobile apps · AI photo recognition · any outbound email · billing or public sign-up · offline sync · recipes and meal planning · home-maintenance scheduling · public sharing links · per-item privacy (all members see everything).

---

## 3. Concepts

- **Family** — the tenant. Many families exist in the schema; one exists in practice.
- **Member** — a `user` (Google account) attached to a family as `owner` or `member`. Both see and edit everything.
- **Person** — *not* a member. Anyone the family tracks things **for**: a child with no account, a grandparent. Persons carry birthdates and size history, and may optionally link to a user.
- **Location → Room → spot** — where a thing physically is. `spot` is free text ("top shelf of the wardrobe").
- **Item** — any possession. May be a **container** (holds other items) and/or a **consumable** (has quantity and/or expiry).
- **Category** — a built-in tree, extendable per family; carries the trait schema.
- **Trait** — a typed, category-specific attribute (size, ISBN, serial number, period-after-opening…).

The **Person / Member** split is deliberate: the second-priority job is kids' sizes, and children will not have Google accounts.

---

## 4. Data model

### Identity and tenancy

```
users(id, google_sub UNIQUE, email, display_name, avatar_url,
      locale NULL, created_at, last_seen_at)

families(id, name, default_currency DEFAULT 'UAH', created_at)

family_members(family_id, user_id, role ENUM(owner, member), joined_at)
  PRIMARY KEY (family_id, user_id)

family_invites(id, family_id, token_hash, created_by, expires_at,
               accepted_at, accepted_by_user_id, revoked_at)

refresh_tokens(id, user_id, token_hash, family_id, issued_at, expires_at,
               rotated_to NULL, revoked_at, user_agent, ip)

people(id, family_id, name, birthdate NULL, user_id NULL,
       is_child, avatar_url NULL, archived_at)

person_sizes(id, person_id, size_type ENUM(clothing, shoe, head),
             size_system, size_value,
             size_norm_min NUMERIC, size_norm_max NUMERIC, recorded_at)
```

### Places

```
locations(id, family_id, name,
          kind ENUM(home, storage, garage, relative, other),
          address NULL, sort_order, archived_at)

rooms(id, location_id, name, sort_order, archived_at)
```

There is no deeper hierarchy. Depth below a room is expressed by `items.spot` and by containers — which keeps the place model simple while still allowing a box to be moved as a unit.

### Taxonomy

```
categories(id, family_id NULL, parent_id NULL, slug, icon,
           name_i18n_key NULL, name_custom NULL, sort_order, archived_at)
    -- family_id NULL -> built-in; label resolved from the i18n bundle
    -- family_id SET  -> family-defined; label is name_custom

trait_defs(id, category_id, family_id NULL, key,
           data_type ENUM(text, number, integer, bool, date, enum, size),
           unit NULL, enum_options JSONB NULL,
           label_i18n_key NULL, label_custom NULL,
           required, filterable, sort_order, archived_at)
```

Built-in categories and their trait definitions are seeded by a **goose Go migration**, so the taxonomy is versioned with the schema. Family-defined categories and custom fields use the same two tables with `family_id` set — one code path, one validator, one renderer.

### Items

```
items(
  id, family_id, name, category_id,

  location_id, room_id NULL, spot NULL,           -- place
  parent_item_id NULL, is_container,              -- containment

  status ENUM(in_use, stored, lent, to_give_away,
              consumed, given_away, sold, discarded, lost),

  quantity NUMERIC DEFAULT 1, unit NULL, min_quantity NULL,   -- consumables
  expires_at NULL, opened_at NULL, pao_months NULL,

  brand NULL, condition ENUM NULL, season ENUM NULL,          -- hot, cross-category
  size_system NULL, size_value NULL,
  size_norm_min NULL, size_norm_max NULL,
  for_person_id NULL,
  serial_number NULL, warranty_until NULL,
  purchase_date NULL, purchase_price NUMERIC NULL, purchase_currency NULL,
  notes NULL,

  traits JSONB NOT NULL DEFAULT '{}',             -- long tail + custom fields
  search_tsv tsvector GENERATED,

  created_by, created_at, updated_at, archived_at
)

tags(id, family_id, name, color)
item_tags(item_id, tag_id)

item_photos(id, item_id, storage_key, thumb_key, width, height, bytes,
            content_type, is_primary, sort_order, uploaded_by, created_at)

item_attachments(id, item_id, kind ENUM(receipt, manual, warranty, other),
                 storage_key, filename, content_type, bytes,
                 uploaded_by, created_at)

item_events(id, family_id, item_id, actor_user_id, type, payload JSONB, created_at)

loans(id, item_id, borrower_person_id NULL, borrower_name NULL,
      lent_at, due_at NULL, returned_at NULL, note)

alert_dismissals(id, family_id, alert_key, item_id NULL,
                 dismissed_by, dismissed_until)

product_cache(barcode PK, source, payload JSONB, fetched_at)
```

#### Why hybrid columns + JSONB, not EAV

EAV in Postgres costs roughly three times the storage and turns any multi-condition filter into a join explosion; JSONB with a GIN index is the established alternative, and the recommended shape is *hot attributes as real columns, long tail in JSONB*.

- **Columns** for anything filtered across categories: size, season, person, expiry, warranty, price, condition, brand, serial.
- **`traits` JSONB** for the long tail (`author`, `isbn`, `voltage`, `dimensions`, `pieces_count`) and for every family-defined custom field.

#### Containment

`parent_item_id` forms a tree. An item's *effective place* is inherited from its root container: moving a container writes one row and emits one event, and descendants resolve their place through the parent. The service layer enforces cycle prevention and a maximum depth of 5.

#### Tenancy invariant

Every sqlc query takes `family_id`, and every handler resolves it from the authenticated session. **A query without a family scope is a review failure**, with no exceptions.

### Built-in category tree (seed)

Each top-level category ships with its trait set and uk/en labels.

| # | Category | Traits |
|---|---|---|
| 1 | Clothing & footwear | size, size_system, season, for_person, color, material, condition, gender_age |
| 2 | Kids' toys | age_min, age_max, for_person, pieces_count, battery_type, condition |
| 3 | Baby & nursery gear | for_person, age_range, safety_expiry, condition |
| 4 | Electronics | model, serial_number, warranty_until, power, connectors, imei, condition |
| 5 | Books & media | author, isbn, language, publisher, year, format, read_status |
| 6 | Kitchen tools & tableware | material, capacity, set_size, dishwasher_safe |
| 7 | Tools & hardware | tool_type, power_source, battery_platform, spec_size |
| 8 | Cosmetics | expires_at, opened_at, pao_months, volume, shade, skin_type, for_person |
| 9 | Hygiene & household chemicals | quantity, unit, min_quantity, expires_at, for_person |
| 10 | Medicines & first aid | expires_at, form, dosage, active_substance, prescription |
| 11 | Food & pantry | expires_at, quantity, unit, storage_kind, opened_at |
| 12 | Textiles & bedding | bedding_size, material, set_size, season |
| 13 | Furniture & interior | dimensions, material, color |
| 14 | Sports & outdoor | activity, size, for_person |
| 15 | Hobby & crafts | craft_type, material, quantity, unit |
| 16 | Office & stationery | kind, quantity, unit |
| 17 | Bags & accessories | kind, material, color, for_person |
| 18 | Auto & bike | vehicle, part_number, size |
| 19 | Garden & balcony | kind, season |
| 20 | Pet supplies | pet, kind, expires_at |
| 21 | Seasonal & decor | occasion, season |
| 22 | Documents & valuables | doc_type, expires_at, issued_by, issued_at |
| 23 | Other | — |

> **Category 22 carries a deliberate restriction:** record *where a document is* and *when it expires* — never passport, ID or account numbers. This keeps a family app out of sensitive-PII territory at almost no cost in usefulness.

### Size normalisation

`size_system` is one of `kids_cm`, `kids_eu`, `intl_letter`, `eu_adult`, `shoe_eu`, `shoe_uk`, `shoe_us`, `one_size`.

A seeded lookup table maps `(system, value) → (norm_min, norm_max)` onto a single numeric scale per `size_type`. A person's current size then matches items by numeric range, so a child on size 110 finds items labelled `110`, `4T` and `104–110` alike.

Unmapped values still store and display normally — they simply do not participate in fit matching. Partial coverage is an acceptable state.

---

## 5. Features by job

### Know what we own

- Browse by location → room, by category tree, by person, by tag, and by container ("what is in box #7").
- Full-text and fuzzy search across name, brand, notes and traits.
- Filter by category traits: size, season, condition, expiry, warranty, price range.
- Duplicate hints — when adding, surface similar existing items in the family.
- Dashboard: counts per location and category, total purchase value, recently added.

### Kids: what fits, what's next

- Person profiles with size history.
- **Fits now** — every item whose normalised size range covers the child's current size, grouped by effective place, including "box #7, garage".
- **Next size up** — what is already owned for the next size, so it is not bought twice.
- **Outgrown** — bulk transition to `to_give_away`.
- **Seasonal pack/unpack** — select a container and flip its contents between `stored` and `in_use` in one action, with one undo.

### Consumables (light)

- Quantity with unit, a `min_quantity` threshold, and quick +/− adjustment.
- Expiry; for cosmetics, `opened_at` + `pao_months` produce a computed effective expiry.
- **Attention panel** (in-app): expired, expiring within 30 days, below minimum, warranty ending within 60 days, loan overdue. Every alert is dismissible for a period.
- Shopping list = everything below minimum, exportable as text.

### Movement and accountability

- Lifecycle states; archival states preserve history rather than deleting it.
- Lending to a person or a free-text outsider, with a due date; overdue loans surface in the Attention panel.
- Item timeline: every create, edit, move and status change, with actor and timestamp.

### Getting data in — the survival feature

- **Batch add** — one screen with shared fields fixed (category, container, location, season) and rows varying only by size, colour or name. Built for "12 bodysuits into box #7". Creates N items in one transaction.
- **Duplicate item** — one tap, then edit the delta.
- **Barcode / ISBN** — camera scan in the browser (`BarcodeDetector`, ZXing-wasm fallback); core-api proxies lookups to Open Food Facts, Open Beauty Facts, Open Products Facts and Google Books, caching responses in `product_cache`. Coverage is good for books, cosmetics, hygiene and food, and patchy elsewhere — the "not found" path must be graceful and fast.
- **CSV import/export** — upload, map columns, dry-run validation report with per-row errors, then commit. Doubles as the backup and the escape hatch.

---

## 6. Architecture

```
Browser ──HTTPS──> Caddy ──> web (Next.js, SSR) ──REST──> core-api (Go) ──> Postgres
                        └──> core-api (/api/v1)                       └──> S3-compatible
(future) Mobile ─────────────────────────────────────────> core-api
```

### core-api owns identity

core-api performs the Google OAuth2 authorization-code exchange (with PKCE), verifies the ID token against Google's JWKS, and issues **its own** tokens: a 15-minute access JWT plus a rotating refresh token, hashed at rest, with reuse detection that revokes the whole token family.

The web app stores them in `httpOnly; Secure; SameSite=Lax` cookies; a future mobile client uses the same endpoints with platform secure storage.

**The web app must never become the identity layer.** If it does, the mobile app inherits a rewrite — which is the whole reason core-api exists.

### Contract-first

`core-api/api/openapi.yaml` (OpenAPI 3.1) is the source of truth for the HTTP contract.

- Go: `oapi-codegen` generates the strict server interface and types.
- Web: `openapi-typescript` generates client types, consumed through a thin fetch wrapper.
- CI fails on generated-code drift.

Edit the spec first, then regenerate. Never hand-edit generated code.

### Internationalisation

- **Taxonomy labels come from the API**, localised via `Accept-Language`, so a future mobile client needs no duplicated translation bundles.
- UI chrome strings live in the web app (`next-intl`, `uk`/`en` bundles).
- API-side strings (validation errors, event descriptions) come from a Go message catalogue.
- Locale resolution order: stored user preference → `NEXT_LOCALE` cookie → GeoIP country (embedded MaxMind GeoLite2-Country; `UA` → `uk`) → `Accept-Language` → `en`. The resolved locale is forwarded to core-api as `Accept-Language`.
- **No user-visible string is ever hardcoded.** Enforced by lint on the web side and by the catalogue on the Go side.

### Photos and attachments

The client resizes to a 1600px long edge and generates a 320px thumbnail, requests presigned PUTs, and uploads directly to object storage, then confirms to the API. The bucket is private; reads go through short-lived presigned GETs. Large files never pass through Go.

### Search

Postgres ships no Ukrainian full-text dictionary, so search is built without stemming:

- `search_tsv`, a generated column using the `simple` configuration over `unaccent(name || brand || notes || traits-as-text)`, with a GIN index — handles tokens and prefixes in both languages.
- A `pg_trgm` GIN index on `name` for fuzzy and substring matching, also powering duplicate hints.

At family scale (10k–100k items) this is comfortably fast. A hunspell `uk` dictionary can be layered on later without a schema change.

### Service structure

`uber-go/fx` modules: `config`, `logging` (slog, JSON), `postgres` (pgxpool), `storage`, `auth`, `catalog`, `items`, `search`, `httpserver`.

Layout follows standard Go: `cmd/api` for the entrypoint, `internal/...` for application code, `db/migrations` (goose) and `db/queries` (sqlc).

---

## 7. Non-functional requirements

**Security** — every query family-scoped; `gosec` in CI; private bucket with presigned URLs only; refresh-token rotation with reuse detection; CSRF protection on cookie-authenticated mutations; strict CSP; rate limits on auth, lookup and upload endpoints.

**Data safety** — nightly `pg_dump` to object storage with a retention policy; a restore that has actually been rehearsed and documented; soft delete (`archived_at`) everywhere; CSV export always available.

**Performance** — search p95 under 300 ms; item list under 1 s on a slow mobile connection.

**Accessibility** — keyboard navigable, labelled form controls, touch targets of at least 44px, WCAG AA contrast.

**Observability** — structured logs with request IDs, `/healthz` and `/readyz`, Postgres and API metrics, an external uptime check.

---

## 8. Acceptance for v1

From a phone: sign in with Google, invite a second member, add a box of 20 real items with photos, move the box to another location, find a specific item by Ukrainian search, and see a correct "fits now" list for a child — with no step requiring a desktop.
