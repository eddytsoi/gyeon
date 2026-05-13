import { parseShortcodes } from './parser';
import { parseProductRef } from './types';

export type ShortcodeRefScan = {
  productIDs: string[];
  productNumbers: number[];
  categorySlugs: string[];
};

function splitCsv(s: string | undefined): string[] {
  if (!s) return [];
  return s.split(',').map((x) => x.trim()).filter(Boolean);
}

// Walk parsed chunks and collect every resource the page will need to fetch
// before render. PRD-N tokens get separated into productNumbers so resolve
// can do a single bulk list lookup instead of N getProductByID calls.
export function scanShortcodeRefs(md: string | undefined | null): ShortcodeRefScan {
  const productIDs = new Set<string>();
  const productNumbers = new Set<number>();
  const categorySlugs = new Set<string>();

  const chunks = parseShortcodes(md);
  for (const c of chunks) {
    if (c.type !== 'shortcode') continue;

    const tokens: string[] = [];
    if (c.name === 'product' && c.attrs.id) tokens.push(c.attrs.id);
    if (c.name === 'products' && c.attrs.ids) tokens.push(...splitCsv(c.attrs.ids));

    for (const token of tokens) {
      const ref = parseProductRef(token);
      if (!ref) continue;
      if (ref.kind === 'uuid') productIDs.add(ref.value);
      else productNumbers.add(ref.value);
    }

    if (c.name === 'products' && c.attrs.categories) {
      for (const slug of splitCsv(c.attrs.categories)) categorySlugs.add(slug);
    }
  }

  return {
    productIDs: [...productIDs],
    productNumbers: [...productNumbers],
    categorySlugs: [...categorySlugs]
  };
}

// Same but across multiple fields (Product has description + how_to_use).
export function scanShortcodeRefsMany(...mds: (string | undefined | null)[]): ShortcodeRefScan {
  const productIDs = new Set<string>();
  const productNumbers = new Set<number>();
  const categorySlugs = new Set<string>();
  for (const md of mds) {
    const s = scanShortcodeRefs(md);
    for (const id of s.productIDs) productIDs.add(id);
    for (const n of s.productNumbers) productNumbers.add(n);
    for (const slug of s.categorySlugs) categorySlugs.add(slug);
  }
  return {
    productIDs: [...productIDs],
    productNumbers: [...productNumbers],
    categorySlugs: [...categorySlugs]
  };
}
