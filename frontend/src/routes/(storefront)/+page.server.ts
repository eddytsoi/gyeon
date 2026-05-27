import { getProducts, getProductImages, getProductVariants, getCmsPageByID } from '$lib/api';
import { scanShortcodeRefs } from '$lib/shortcodes/scan';
import { resolveShortcodeRefs } from '$lib/shortcodes/resolve';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, cookies }) => {
  const { publicSettings } = await parent();
  const homepageId = publicSettings?.find((s) => s.key === 'homepage_page_id')?.value ?? '';

  if (homepageId) {
    const page = await getCmsPageByID(homepageId).catch(() => null);
    if (page) {
      const shortcodeRefs = await resolveShortcodeRefs(scanShortcodeRefs(page.content));
      return { mode: 'page' as const, page, shortcodeRefs };
    }
  }

  // Forward customer_token so the fallback product grid shows the role-
  // correct subset (e.g. installer sees installer-only categories).
  const token = cookies.get('customer_token') ?? null;
  const products = (await getProducts(8, 0, '', token).catch(() => [])) ?? [];
  const enriched = await Promise.all(
    products.map(async (product) => {
      const [variants, images] = await Promise.all([
        getProductVariants(product.id).catch(() => []),
        getProductImages(product.id).catch(() => [])
      ]);
      return {
        product,
        primaryImage: images.find((i) => i.is_primary) ?? images[0],
        cheapestVariant: variants.sort((a, b) => a.price - b.price)[0]
      };
    })
  );

  return { mode: 'default' as const, products: enriched };
};
