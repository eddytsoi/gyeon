import type { Product, ProductImage, Variant } from '$lib/types';

export type ShortcodeAttrs = Record<string, string>;

export type Chunk =
  | { type: 'md'; text: string }
  | { type: 'shortcode'; name: string; attrs: ShortcodeAttrs; body: string; raw: string };

export type ShortcodeProductRef = {
  product: Product;
  image: ProductImage | null;
  variant: Variant | null;
};

// FormFieldOption / PublicForm mirror backend/internal/forms types — kept in
// sync by hand. Update both sides if the field schema changes.
export type FormFieldOption = { label: string; value: string };

export type FormField = {
  type:
    | 'text'
    | 'email'
    | 'tel'
    | 'textarea'
    | 'select'
    | 'checkbox'
    | 'radio'
    | 'date'
    | 'submit'
    | 'hidden';
  name: string;
  required?: boolean;
  label?: string;
  placeholder?: string;
  default?: string;
  id?: string;
  class?: string;
  size?: number;
  maxlength?: number;
  minlength?: number;
  min?: string;
  max?: string;
  options?: FormFieldOption[];
};

export type PublicForm = {
  id: string;
  slug: string;
  title: string;
  fields: FormField[];
  success_message: string;
  error_message: string;
  recaptcha_action: string;
};

export type ShortcodeRefs = {
  // Card data keyed by product UUID — the canonical lookup the components
  // dispatch into. PRD-N and category slugs both resolve through this map.
  products: Record<string, ShortcodeProductRef>;
  // Number → UUID map so [product id="PRD-181"] and [products ids="PRD-1,PRD-2"]
  // can find their target without exposing raw UUIDs to authors.
  productsByNumber: Record<number, string>;
  // Slug → ordered UUID list. Order is the natural order returned by the
  // category list endpoint (newest first today).
  productsByCategory: Record<string, string[]>;
  // Forms keyed by slug — populated by +page.server.ts when the page
  // embeds one or more `[contact-form id="..."]` shortcodes.
  forms: Record<string, PublicForm>;
};

export const EMPTY_REFS: ShortcodeRefs = {
  products: {},
  productsByNumber: {},
  productsByCategory: {},
  forms: {}
};

export const KNOWN_SHORTCODES = ['product', 'products', 'button', 'note', 'section', 'banner', 'contact-form'] as const;
export type KnownShortcode = (typeof KNOWN_SHORTCODES)[number];

export function isKnownShortcode(name: string): name is KnownShortcode {
  return (KNOWN_SHORTCODES as readonly string[]).includes(name);
}

// A product reference written as either a UUID or `PRD-<number>` (case-
// insensitive). Returns null for an empty/whitespace-only token so callers
// can skip it without special-casing.
export type ProductRef = { kind: 'uuid'; value: string } | { kind: 'number'; value: number };

export function parseProductRef(token: string | undefined | null): ProductRef | null {
  if (!token) return null;
  const t = token.trim();
  if (!t) return null;
  const m = /^PRD-(\d+)$/i.exec(t);
  if (m) return { kind: 'number', value: Number(m[1]) };
  return { kind: 'uuid', value: t };
}

// Resolve a PRD-N / UUID token to a UUID using the refs map. Returns null
// when the number isn't in the resolved set (typo, hidden product, etc.).
export function resolveProductRef(token: string, refs: ShortcodeRefs): string | null {
  const ref = parseProductRef(token);
  if (!ref) return null;
  if (ref.kind === 'uuid') return ref.value;
  return refs.productsByNumber[ref.value] ?? null;
}
