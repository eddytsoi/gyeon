import { getProducts, getProductImages, getProductVariants, getCategories } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  const [products, categories] = await Promise.all([
    getProducts(100, 0).catch(() => []),
    getCategories().catch(() => [])
  ]);

  const product = products.find((p) => p.slug === params.slug);
  if (!product) throw error(404, 'Product not found');

  const category = categories.find((c) => c.id === product.category_id) ?? null;

  const related = products
    .filter((p) => p.id !== product.id && p.category_id === product.category_id && p.status === 'active')
    .slice(0, 4);

  const [variants, images, ...relatedImages] = await Promise.all([
    getProductVariants(product.id).catch(() => []),
    getProductImages(product.id).catch(() => []),
    ...related.map((p) => getProductImages(p.id).catch(() => []))
  ]);

  const relatedWithImage = related.map((p, i) => ({
    ...p,
    primaryImage: relatedImages[i]?.find((img) => img.is_primary) ?? relatedImages[i]?.[0] ?? null
  }));

  return { product, variants, images, category, related: relatedWithImage };
};
