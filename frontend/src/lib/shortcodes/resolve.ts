import { getProductByID, getProductImages, getProductVariants } from '$lib/api';
import type { ShortcodeRefs, ShortcodeProductRef } from './types';
import type { ShortcodeRefScan } from './scan';

// Fetch every resource referenced by shortcodes in parallel. Failures are
// swallowed per-id so a broken shortcode doesn't 500 the whole page — the
// component just renders nothing for missing refs.
export async function resolveShortcodeRefs(scan: ShortcodeRefScan): Promise<ShortcodeRefs> {
  const products: Record<string, ShortcodeProductRef> = {};

  if (scan.productIDs.length > 0) {
    const entries = await Promise.all(
      scan.productIDs.map(async (id) => {
        const product = await getProductByID(id).catch(() => null);
        if (!product) return null;
        const [images, variants] = await Promise.all([
          getProductImages(id).catch(() => []),
          getProductVariants(id).catch(() => [])
        ]);
        const image = images.find((i) => i.is_primary) ?? images[0] ?? null;
        const variant = variants.find((v) => v.is_active) ?? variants[0] ?? null;
        return [id, { product, image, variant }] as const;
      })
    );
    for (const entry of entries) {
      if (entry) products[entry[0]] = entry[1];
    }
  }

  return { products };
}
