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
| `dilution` | ratios stated in `description` / `how_to_use` only — **never invented** |
| `compatible`, `incompatible` | `適用表面` / `適用於…` text and 注意事項 |
| `frequency` | 保養建議 if stated, else inferred (flagged) |
| `pairedWith` | seeded from `upsells` (`upsell`), `frequently-bought-together` (`fbt`), and products linked inside `how_to_use` (`usage`); plus editorial KB suggestions (`editorial`) |
| `variants`, `wcSku` | `variants` endpoint (`wc_sku` = barcode; `wcSku` set only when single-variant) |
| `faq`, `seo` | generated from product content (always flagged for review) |
| `imageUrl` | `primary_image_url` |

## `_meta` — trust signals

Every record carries `_meta`:

- **`needsReview`** — list of fields that were inferred or generated (not lifted verbatim from the
  source). Always includes `faq` and `seo`; includes `frequency` and `dilution` when the source did not
  state them. **Review these before treating the field as authoritative.**
- **`confidence`** — `high` (mostly grounded in source), `medium`, or `low`.
- **`sources`** — which source fields the record drew from.

> **Safety note:** `dilution` ratios are only present when the official source stated them. An empty
> `dilution: {}` with `"dilution"` in `needsReview` means "not specified" — do **not** assume a ratio.

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
