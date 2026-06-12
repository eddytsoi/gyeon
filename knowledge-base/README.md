# GYEON HK Product Knowledge Base

A single structured **source of truth** for every GYEON HK product. Built to drive three things:

1. **Mix-and-match pairings** — `pairedWith` per product (incl. upcoming products, appended later).
2. **SEO / GEO** — `seo`, `faq`, `keyFeatures`, `usage` feed search + AI answer-engine structured data.
3. **Internal knowledge base** — one canonical reference per SKU.

## Files

| File | What |
|---|---|
| `products.json` | The data — a wrapper (`version`, `generatedAt`, `source`, `currency`, `language`, `productCount`) + `products[]`. |
| `schema.json` | JSON Schema (draft 2020-12) defining and validating every record. |
| `README.md` | This file. |

## The join key

`sku` **= the storefront product slug**, and `url` = `/products/{sku}`. So every record joins 1:1 to:
- the live storefront page (`/products/{sku}`),
- the DB (`products.slug`, plus `productId` = `products.id`),
- `pairedWith[].sku` and `variants[].sku` — both reference real product/variant SKUs in the catalog.

Generated from the public storefront API (`https://gyeon.hk/api/v1`): product list + detail
(`description`, `how_to_use`, `subtitle`, `excerpt`), `variants` (SKU, barcode, price, size),
`upsells`, and `frequently-bought-together`.

## Field provenance

| Field | Source |
|---|---|
| `sku`, `name`, `number`, `productId`, `url` | storefront product record (verbatim) |
| `category`, `categories` | `category_id` / `category_ids` → category name |
| `tagline` | `subtitle` (verbatim) |
| `shortDescription` | `excerpt` (verbatim) |
| `description` | `description` markdown → clean 繁中 text |
| `keyFeatures` | extracted from `description` |
| `usage.steps`, `usage.cautions` | parsed from `how_to_use` (使用步驟 / 注意事項) |
| `dilution`, `dilutable`, `dilutionNote` | ratios stated in source / official GYEON pages only — **never invented**; `dilutable` + note classify every product (see below) |
| `compatible`, `incompatible` | `適用表面` / `適用於…` text and 注意事項; ambiguous liquids verified against gyeonusa.com |
| `frequency` | 保養建議 if stated, else inferred (flagged) |
| `pairedWith` | seeded from `upsells` (`upsell`), `frequently-bought-together` (`fbt`), and products linked inside `how_to_use` (`usage`); plus editorial KB suggestions (`editorial`) |
| `variants`, `wcSku` | `variants` endpoint (`wc_sku` = barcode; `wcSku` set only when single-variant) |
| `faq`, `seo` | generated from product content (always flagged for review) |
| `imageUrl` | `primary_image_url` |

## `_meta` — trust signals

Every record carries `_meta`:

- **`needsReview`** — fields still unresolved. **Empty once a product has been reviewed**; a residual
  entry means it could not be verified (its `reviewed` entry will be `confidence:"low"`).
- **`confidence`** — overall = the **lowest** field confidence in `reviewed`.
- **`sources`** — which sources the record drew from (incl. official `gyeonusa.com` URLs).
- **`reviewed`** — per-field audit map, `field → {confidence, basis, source?, note?}`, recording how each
  field was checked. `basis` is one of:
  - **`official`** — verified against an official GYEON page (`source` URL).
  - **`not-applicable`** — the field has no meaning for this product type (tool / PPF film / bundle).
  - **`editorial`** — a curated judgment (e.g. `pairedWith` mix-and-match suggestions).
  - **`source-derived`** — kept from the gyeon.hk source content (no official page to verify against).
  - **`consistency-check`** — `faq` checked against the record's verified facts.
  - **`mechanical-check`** — `seo` checked against length / keyword-count rules.

## `dilutable` — dilution status (no ambiguity)

Every product carries a **`dilutable`** boolean + a 繁中 **`dilutionNote`**, so an empty `dilution` is never
mistaken for missing data:

- **`dilutable: true`** — the product is diluted before use; `dilution` holds the ratio(s)
  (e.g. `{"bucket":"500:1","foamGun":"1:15"}`). Only the wash shampoos and Q²M Preserve.
- **`dilutable: false`** — `dilution: {}` with a `dilutionNote` explaining why: `即用，無需稀釋`
  (ready-to-use liquid), `原液使用，無需稀釋` (coating, applied neat), or `不適用` (tool / film / bundle).

The GYEON Q²M line is overwhelmingly ready-to-use; the ambiguous liquids were each verified against the
official **gyeonusa.com** product page (added to `_meta.sources`). No product still carries `"dilution"`
in `needsReview`.

> **Safety note:** dilution ratios are only present when an official source stated them. Empty
> `dilution: {}` means **no dilution needed** (`dilutable:false`) — do not assume a ratio.

## How each use case consumes it

- **Mix-and-match:** read `pairedWith` (filter by `relation`; `editorial` = curated suggestions).
  For upcoming products, add the record + cross-link its `pairedWith` into the existing SKUs.
- **SEO / GEO:** map `seo` → `<title>`/meta, `faq` → `FAQPage` JSON-LD, `usage` → `HowTo` JSON-LD,
  `keyFeatures`/`description` → Product schema. (The PDP already emits Product/Offer/BreadcrumbList.)
- **Internal KB:** the whole record per `sku`; `compatible`/`incompatible`/`dilution`/`frequency`
  answer the common "can I use X on Y / how do I dilute / how often" questions.

## Regenerating

Raw API data is pulled per product, then each product is curated into a record against `schema.json`.
Validate after any change:

```bash
ajv validate -s knowledge-base/schema.json -d knowledge-base/products.json   # or any draft-2020-12 validator
jq '.productCount == (.products|length)' knowledge-base/products.json          # must be true
```
