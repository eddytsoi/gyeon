import {
  getProductByID,
  getProductImages,
  getProductVariants,
  getProducts,
  getProductsByCategorySlug
} from '$lib/api';
import type { ShortcodeRefs, ShortcodeProductRef } from './types';
import type { ShortcodeRefScan } from './scan';

// Cap how many products a single [products categories="..."] expansion can
// pull per category. Authors who need bigger grids can pass `limit`, but
// without a cap a popular category could drop hundreds of cards into a post.
const DEFAULT_PER_CATEGORY = 12;

// Cap the per-number bulk-lookup pool. PRD-N resolution scans the public
// product list once per page render; this bounds the fetch size.
const NUMBER_LOOKUP_POOL = 500;

async function fetchProductRef(id: string): Promise<ShortcodeProductRef | null> {
  const product = await getProductByID(id).catch(() => null);
  if (!product) return null;
  const [images, variants] = await Promise.all([
    getProductImages(id).catch(() => []),
    getProductVariants(id).catch(() => [])
  ]);
  const image = images.find((i) => i.is_primary) ?? images[0] ?? null;
  const variant = variants.find((v) => v.is_active) ?? variants[0] ?? null;
  return { product, image, variant };
}

// Fetch every resource referenced by shortcodes in parallel. Failures are
// swallowed per-id so a broken shortcode doesn't 500 the whole page — the
// component just renders nothing for missing refs.
export async function resolveShortcodeRefs(scan: ShortcodeRefScan): Promise<ShortcodeRefs> {
  const products: Record<string, ShortcodeProductRef> = {};
  const productsByNumber: Record<number, string> = {};
  const productsByCategory: Record<string, string[]> = {};

  // 1. Resolve PRD-N references via the public product list. One call covers
  //    every number on the page; we then dedupe against UUID resolution so we
  //    don't fetch the same product twice.
  const numberSet = new Set(scan.productNumbers);
  let numberLookup: Map<number, string> = new Map();
  if (numberSet.size > 0) {
    const list = await getProducts(NUMBER_LOOKUP_POOL, 0).catch(() => []);
    for (const p of list) {
      if (numberSet.has(p.number)) numberLookup.set(p.number, p.id);
    }
  }

  // 2. Collect every UUID we need card data for: explicit UUIDs, UUIDs that
  //    PRD-N resolved to, and UUIDs from category expansion (below).
  const uuidsToFetch = new Set<string>(scan.productIDs);
  for (const uuid of numberLookup.values()) uuidsToFetch.add(uuid);

  // 3. Resolve category slugs. Each slug returns an ordered UUID list; we
  //    also queue each UUID for card-data fetch.
  if (scan.categorySlugs.length > 0) {
    const lists = await Promise.all(
      scan.categorySlugs.map((slug) =>
        getProductsByCategorySlug(slug, DEFAULT_PER_CATEGORY, 0).catch(() => [])
      )
    );
    scan.categorySlugs.forEach((slug, i) => {
      const ids = lists[i].map((p) => p.id);
      productsByCategory[slug] = ids;
      for (const id of ids) uuidsToFetch.add(id);
    });
  }

  // 4. Fetch image + variant for every needed UUID in parallel.
  if (uuidsToFetch.size > 0) {
    const ids = [...uuidsToFetch];
    const refs = await Promise.all(ids.map((id) => fetchProductRef(id)));
    ids.forEach((id, i) => {
      const ref = refs[i];
      if (ref) products[id] = ref;
    });
  }

  // 5. Build the number → UUID map, but only for products that actually have
  //    card data — a number that resolved to a UUID but failed image/variant
  //    fetch shouldn't appear.
  for (const [num, uuid] of numberLookup) {
    if (products[uuid]) productsByNumber[num] = uuid;
  }

  return { products, productsByNumber, productsByCategory };
}
