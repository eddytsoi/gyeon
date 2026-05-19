import { getProductBySlug, getProducts, getProductImages, getProductVariants, getCategories, getProductBundleItems, getProductPromoBundles, getPublicSettings } from '$lib/api';
import { error } from '@sveltejs/kit';
import { scanShortcodeRefsMany } from '$lib/shortcodes/scan';
import { resolveShortcodeRefs } from '$lib/shortcodes/resolve';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  // Direct slug lookup — bypasses the hidden-category filter so hidden
  // products remain reachable via direct URL / private links.
  const product = await getProductBySlug(params.slug).catch(() => null);
  if (!product) throw error(404, 'Product not found');

  const [products, categories] = await Promise.all([
    getProducts(100, 0).catch(() => []),
    getCategories().catch(() => [])
  ]);

  const category = categories.find((c) => c.id === product.category_id) ?? null;

  // Related products come from the public list, which already excludes
  // hidden products, so a hidden product's page won't surface other
  // hidden products as "related". Shuffle so each PDP load surfaces a
  // different mix from the same primary category.
  const pool = products.filter(
    (p) =>
      p.id !== product.id &&
      p.category_id === product.category_id &&
      p.status === 'active' &&
      (p.default_variant_stock_qty ?? 0) > 0
  );
  for (let i = pool.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [pool[i], pool[j]] = [pool[j], pool[i]];
  }
  const related = pool.slice(0, 4);

  const [variants, images, bundleItems, promoBundles, settings, shortcodeRefs, ...relatedImages] = await Promise.all([
    getProductVariants(product.id).catch(() => []),
    getProductImages(product.id).catch(() => []),
    product.kind === 'bundle' ? getProductBundleItems(product.id).catch(() => []) : Promise.resolve([]),
    // Promo bundles only make sense for non-bundle parents. Bundles can't host them.
    product.kind !== 'bundle' ? getProductPromoBundles(product.id).catch(() => []) : Promise.resolve([]),
    getPublicSettings().catch(() => []),
    resolveShortcodeRefs(scanShortcodeRefsMany(product.description, product.how_to_use)),
    ...related.map((p) => getProductImages(p.id).catch(() => []))
  ]);

  const relatedWithImage = related.map((p, i) => ({
    ...p,
    primaryImage: relatedImages[i]?.find((img) => img.is_primary) ?? relatedImages[i]?.[0] ?? null
  }));

  // Per-product `use_taobao_layout` (true / false) wins over the site
  // default; null/undefined falls through to the site setting. Bundles
  // never use the taobao layout — they have no variants and no promo
  // bundles to surface.
  const siteTaobaoOn = settings.find((s) => s.key === 'pdp_taobao_layout_enabled')?.value === 'true';
  const useTaobaoLayout =
    product.kind !== 'bundle' &&
    (product.use_taobao_layout === true ||
      (product.use_taobao_layout == null && siteTaobaoOn));

  return {
    product,
    variants,
    images,
    bundleItems,
    promoBundles,
    category,
    related: relatedWithImage,
    shortcodeRefs,
    useTaobaoLayout
  };
};
