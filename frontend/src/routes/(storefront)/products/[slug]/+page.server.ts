import { getProductBySlug, getProducts, getProductImages, getProductVariants, getCategories, getProductBundleItems } from '$lib/api';
import { error } from '@sveltejs/kit';
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
  // hidden products as "related".
  const related = products
    .filter((p) => p.id !== product.id && p.category_id === product.category_id && p.status === 'active')
    .slice(0, 4);

  const [variants, images, bundleItems, ...relatedImages] = await Promise.all([
    getProductVariants(product.id).catch(() => []),
    getProductImages(product.id).catch(() => []),
    product.kind === 'bundle' ? getProductBundleItems(product.id).catch(() => []) : Promise.resolve([]),
    ...related.map((p) => getProductImages(p.id).catch(() => []))
  ]);

  const relatedWithImage = related.map((p, i) => ({
    ...p,
    primaryImage: relatedImages[i]?.find((img) => img.is_primary) ?? relatedImages[i]?.[0] ?? null
  }));

  return { product, variants, images, bundleItems, category, related: relatedWithImage };
};
