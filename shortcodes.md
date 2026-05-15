# Shortcodes

WordPress-style `[name attr="value"]…[/name]` tags that admins can drop into any markdown body the storefront renders — CMS pages (`/admin/cms/pages`), blog posts (`/admin/cms/posts`), product long descriptions, etc. They are parsed and rendered client-side by `frontend/src/lib/components/MarkdownContent.svelte`; the Go backend stores them as opaque text.

## Syntax

- Tag names are **case-insensitive ASCII** (`[a-z][a-z0-9_-]*`). `[Note]` and `[note]` are the same tag.
- Attribute values must be **double-quoted**. Single quotes and unquoted values do not parse: `[note type="info"]` ✓, `[note type=info]` ✗.
- Attribute names are also lowercased — `Class="x"` becomes `class="x"`.
- A tag may be **paired** (`[note]…[/note]`) or **self-closing** (`[banner image="…"]`). The renderer picks the right one per shortcode; if a paired tag has no matching closer, it renders as self-closing with an empty body.
- Same-name tags **nest** — `[section][section]inner[/section][/section]` matches the outer closer first.
- Escape literal brackets with a backslash: `\[note]` renders as the four characters `[note]`. `\\` becomes a single `\`.
- Unknown shortcode names are left **verbatim** in the source (WordPress behavior) so you can paste markdown that references third-party tags without the parser eating it.

## The universal `class` attribute

Every shortcode accepts an optional `class="..."` attribute. The string is **appended** to the rendered container's existing classes — it never replaces them. Use it to layer Tailwind utilities, brand-specific class hooks, or one-off CSS without editing component source.

```
[note type="warn" class="my-test-cls border-pink-500"]Heads up.[/note]
```

The rendered `<div>` gets every default class (`my-6 rounded-xl border-l-4 px-5 py-4 prose prose-sm max-w-none border-amber-300 bg-amber-50 text-amber-900`) plus `my-test-cls border-pink-500` at the end. Tailwind CSS specificity rules then determine which wins.

For `[section]` and `[banner]` (which delegate to wrapper components) the class lands on the **outer** element — the `<section>` and the `<a>` / `<div>` respectively. For `[contact-form]` it's applied to whichever root the current state renders (form / fallback notice / success message), so the class always shows up.

## Reference

### `[product]`

Renders a single product card.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `id` | yes | Product UUID **or** `PRD-<number>` | — |
| `class` | no | any string | — |

```
[product id="22222222-0000-0000-0000-000000000003"]
[product id="PRD-7" class="my-12"]
```

If the id can't be resolved, the shortcode renders nothing.

---

### `[products]`

Renders a responsive grid (`2 / 3 / 4` columns) of product cards. Order: explicit `ids` first (in author-declared order), then the `categories` expansion. Duplicate UUIDs are dropped.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `ids` | one of `ids`/`categories` | comma-separated UUIDs / `PRD-<n>` | — |
| `categories` | one of `ids`/`categories` | comma-separated category slugs | — |
| `limit` | no | positive integer | `12` |
| `class` | no | any string | — |

```
[products ids="PRD-1,PRD-2,PRD-3"]
[products categories="coating,detailing" limit="8"]
[products ids="PRD-1" categories="essentials" limit="6" class="bg-paper p-6 rounded-2xl"]
```

If no items resolve, the shortcode renders nothing.

---

### `[button]`

Renders a styled link-button.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `href` | yes | any URL or path | — |
| `label` | yes | any string | — |
| `style` | no | `primary` \| `secondary` | `primary` |
| `rounded` | no | `sm` \| `md` \| `xl` | `xl` |
| `size` | no | integer 8–96 (px) | `14` |
| `font-weight` | no | integer 100–900 | `600` |
| `color` | no | hex `#RGB` or `#RRGGBB` | _(theme default)_ |
| `class` | no | any string | — |

```
[button href="/products" label="Shop now"]
[button href="https://example.com" label="Read more" style="secondary" rounded="md" size="16" font-weight="700" color="#1a1a1a"]
```

Out-of-range or malformed values fall back to the default; in dev a `console.warn` flags the issue. The shortcode renders nothing if `href` or `label` is missing.

---

### `[note]`

Tone-tinted callout box. Body content supports markdown (and nested shortcodes).

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `type` | no | `info` \| `warn` \| `success` | `info` |
| `class` | no | any string | — |

```
[note]Informational by default.[/note]
[note type="warn"]Watch out.[/note]
[note type="success" class="text-base"]Order placed.[/note]
```

Tone palettes: `info` → sky, `warn` → amber, `success` → emerald.

---

### `[section]`

Editorial layout block — full-bleed background, configurable layout, container padding, and an optional `id` for in-page anchors. Body supports markdown and nested shortcodes; with split layouts, a markdown horizontal rule (`---` on its own line) separates the two halves.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `bg` | no | `paper` \| `cream` \| `white` \| `ink-900` \| `navy-900` | `paper` |
| `layout` | no | `default` \| `split` \| `split-reverse` \| `hero` | `default` |
| `padding` | no | `sm` \| `md` \| `lg` | `md` |
| `width` | no | `default` \| `narrow` \| `full` | `default` |
| `align` | no | `left` \| `center` | `left` |
| `bleed` | no | `full` \| `container` | `full` |
| `id` | no | any string (rendered as HTML `id`) | — |
| `class` | no | any string | — |

```
[section bg="cream" align="center" id="features"]
## Why Gyeon
Body markdown here.
[/section]

[section layout="split" bg="paper"]
## Left side
Copy on the left.

---

![right hero](/uploads/hero.jpg)
[/section]
```

Width/padding/bg/bleed maps live in `frontend/src/lib/shortcodes/section.ts`.

---

### `[banner]`

Edge-to-edge image banner, optionally hyperlinked. Renders as `<a>` if `href` is set, else `<div>`. Supports separate mobile artwork and per-breakpoint aspect ratio.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `image` | yes | image URL or `/uploads/...` path | — |
| `image-mobile` | no | image URL (used at `<768px`) | — |
| `alt` | no | alt text (recommended for accessibility) | `""` |
| `href` | no | URL or path | — |
| `bleed` | no | `full` \| `container` | `full` |
| `bleed-lg` | no | `full` \| `container` (overrides at ≥1024px) | _(inherits `bleed`)_ |
| `aspect-ratio` | no | positive number (e.g. `1.78`) or `auto` | `auto` |
| `aspect-ratio-mobile` | no | positive number or `auto` | `auto` |
| `height` | no | integer 1–2000 (px) or `auto` | `auto` |
| `class` | no | any string | — |

```
[banner image="/uploads/hero.jpg" alt="New collection"]
[banner image="/uploads/desktop.jpg" image-mobile="/uploads/mobile.jpg" alt="Sale" href="/sale" aspect-ratio="2.4" aspect-ratio-mobile="1" bleed="full" bleed-lg="container"]
```

If `image` is missing the shortcode renders nothing. In dev, a missing/empty `alt` logs a warning unless explicitly set to `alt=""` for decorative images.

---

### `[contact-form]`

Renders a form defined under `/admin/forms`. Fields, validation, and submission are server-driven by the form definition referenced by `id` (the form's slug). Optional reCAPTCHA v3 honors the `recaptcha_enabled` site setting.

| Attribute | Required | Allowed values | Default |
|---|---|---|---|
| `id` | yes | form slug | — |
| `class` | no | any string | — |

```
[contact-form id="general-enquiry"]
[contact-form id="press" class="max-w-2xl mx-auto"]
```

If the slug doesn't resolve to a form, a dashed-border placeholder is shown instead — the `class` still applies, so the placeholder slots into the surrounding layout the same way the form would.

## Editor toolbar

The admin CMS editor has a **Shortcode** toolbar (`frontend/src/lib/components/admin/ShortcodeToolbar.svelte`) that inserts these tags pre-filled. Use it when authoring instead of typing by hand to avoid quoting / nesting mistakes.

## Adding a new shortcode

1. Add the tag name to `KNOWN_SHORTCODES` in `frontend/src/lib/shortcodes/types.ts` so the parser recognises it.
2. Create `frontend/src/lib/components/shortcodes/<Name>Shortcode.svelte`. Read `attrs` (and `body`/`refs` if needed) via `$props()`. Append `attrs.class` to the root container's class string to keep the universal class behaviour.
3. Register the component in `frontend/src/lib/shortcodes/registry.ts`.
4. If the shortcode needs server-fetched data (products, forms, etc.), extend the scan/resolve pipeline in `frontend/src/lib/shortcodes/scan.ts` and `resolve.ts` so the data is prefetched before render.
5. Add a row to this file's reference table.
