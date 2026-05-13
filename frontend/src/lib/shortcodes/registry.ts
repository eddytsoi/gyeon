import type { Component } from 'svelte';
import ProductShortcode from '$lib/components/shortcodes/ProductShortcode.svelte';
import ProductsShortcode from '$lib/components/shortcodes/ProductsShortcode.svelte';
import ButtonShortcode from '$lib/components/shortcodes/ButtonShortcode.svelte';
import NoteShortcode from '$lib/components/shortcodes/NoteShortcode.svelte';
import SectionShortcode from '$lib/components/shortcodes/SectionShortcode.svelte';

// The single place that maps shortcode names to their renderer components.
// Add a new shortcode by importing its component here and adding it to the
// map plus the KNOWN_SHORTCODES list in types.ts.
export const shortcodeRegistry: Record<string, Component<any>> = {
  product: ProductShortcode,
  products: ProductsShortcode,
  button: ButtonShortcode,
  note: NoteShortcode,
  section: SectionShortcode
};
