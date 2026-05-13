import { parseShortcodes } from './parser';

export type ShortcodeRefScan = {
  productIDs: string[];
};

// Walk parsed chunks and collect the unique resource IDs referenced by
// known shortcodes. Drives the SSR-time bulk fetch in resolve.ts so the
// page renders synchronously once the load function returns.
export function scanShortcodeRefs(md: string | undefined | null): ShortcodeRefScan {
  const productIDs = new Set<string>();
  const chunks = parseShortcodes(md);
  for (const c of chunks) {
    if (c.type !== 'shortcode') continue;
    if (c.name === 'product' && c.attrs.id) {
      productIDs.add(c.attrs.id);
    } else if (c.name === 'products' && c.attrs.ids) {
      for (const id of c.attrs.ids.split(',')) {
        const trimmed = id.trim();
        if (trimmed) productIDs.add(trimmed);
      }
    }
  }
  return { productIDs: [...productIDs] };
}

// Same but across multiple fields (Product has description + how_to_use).
export function scanShortcodeRefsMany(...mds: (string | undefined | null)[]): ShortcodeRefScan {
  const productIDs = new Set<string>();
  for (const md of mds) {
    for (const id of scanShortcodeRefs(md).productIDs) productIDs.add(id);
  }
  return { productIDs: [...productIDs] };
}
