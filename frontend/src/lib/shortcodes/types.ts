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

export type ShortcodeRefs = {
  products: Record<string, ShortcodeProductRef>;
};

export const EMPTY_REFS: ShortcodeRefs = { products: {} };

export const KNOWN_SHORTCODES = ['product', 'products', 'button', 'note'] as const;
export type KnownShortcode = (typeof KNOWN_SHORTCODES)[number];

export function isKnownShortcode(name: string): name is KnownShortcode {
  return (KNOWN_SHORTCODES as readonly string[]).includes(name);
}
