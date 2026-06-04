import { parseShortcodes } from './parser';
import { parseProductRef } from './types';

export type ShortcodeRefScan = {
  productIDs: string[];
  productNumbers: number[];
  categorySlugs: string[];
  // Per-slug fetch cap derived from the max of limit / limit-md / limit-lg
  // across every [products categories="..."] shortcode on the page.
  categoryLimits: Record<string, number>;
  formSlugs: string[];
  mediaNames: string[];
};

// Tokens that already look like a URL/path bypass the media-by-name lookup —
// the [photo-grid] component will use them verbatim. Keeps backward compat
// with authors who paste a /uploads/... path from the admin Media picker.
function isUrlLikeMediaToken(token: string): boolean {
  return token.startsWith('/') || /^https?:\/\//i.test(token);
}

function splitCsv(s: string | undefined): string[] {
  if (!s) return [];
  return s.split(',').map((x) => x.trim()).filter(Boolean);
}

// Walk parsed chunks and collect every resource the page will need to fetch
// before render. PRD-N tokens get separated into productNumbers so resolve
// can do a single bulk list lookup instead of N getProductByID calls.
// Recurses into shortcode bodies so refs nested inside wrappers like
// [section]…[product …][/section] still get pre-fetched.
function parseAttrLimit(val: string | undefined, fallback: number): number {
  return val && /^\d+$/.test(val) ? Math.max(1, Number(val)) : fallback;
}

export function scanShortcodeRefs(md: string | undefined | null): ShortcodeRefScan {
  const productIDs = new Set<string>();
  const productNumbers = new Set<number>();
  const categorySlugs = new Set<string>();
  const categoryLimits: Record<string, number> = {};
  const formSlugs = new Set<string>();
  const mediaNames = new Set<string>();

  function walk(src: string | undefined | null) {
    const chunks = parseShortcodes(src);
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
        const baseLim    = parseAttrLimit(c.attrs.limit,       12);
        const smLim      = parseAttrLimit(c.attrs['limit-sm'], baseLim);
        const tabletLim  = parseAttrLimit(c.attrs['limit-md'], smLim);
        const desktopLim = parseAttrLimit(c.attrs['limit-lg'], tabletLim);
        const maxLim = Math.max(baseLim, smLim, tabletLim, desktopLim);
        for (const slug of splitCsv(c.attrs.categories)) {
          categorySlugs.add(slug);
          categoryLimits[slug] = Math.max(categoryLimits[slug] ?? 0, maxLim);
        }
      }

      if (c.name === 'contact-form' && c.attrs.id) {
        formSlugs.add(c.attrs.id);
      }

      if (c.name === 'photo-grid' && c.attrs.source) {
        for (const token of splitCsv(c.attrs.source)) {
          if (!isUrlLikeMediaToken(token)) mediaNames.add(token);
        }
      }

      if (c.body) walk(c.body);
    }
  }

  walk(md);

  return {
    productIDs: [...productIDs],
    productNumbers: [...productNumbers],
    categorySlugs: [...categorySlugs],
    categoryLimits,
    formSlugs: [...formSlugs],
    mediaNames: [...mediaNames]
  };
}

// Same but across multiple fields (Product has description + how_to_use).
export function scanShortcodeRefsMany(...mds: (string | undefined | null)[]): ShortcodeRefScan {
  const productIDs = new Set<string>();
  const productNumbers = new Set<number>();
  const categorySlugs = new Set<string>();
  const formSlugs = new Set<string>();
  const mediaNames = new Set<string>();
  const categoryLimits: Record<string, number> = {};
  for (const md of mds) {
    const s = scanShortcodeRefs(md);
    for (const id of s.productIDs) productIDs.add(id);
    for (const n of s.productNumbers) productNumbers.add(n);
    for (const slug of s.categorySlugs) categorySlugs.add(slug);
    for (const [slug, lim] of Object.entries(s.categoryLimits)) {
      categoryLimits[slug] = Math.max(categoryLimits[slug] ?? 0, lim);
    }
    for (const slug of s.formSlugs) formSlugs.add(slug);
    for (const name of s.mediaNames) mediaNames.add(name);
  }
  return {
    productIDs: [...productIDs],
    productNumbers: [...productNumbers],
    categorySlugs: [...categorySlugs],
    categoryLimits,
    formSlugs: [...formSlugs],
    mediaNames: [...mediaNames]
  };
}
