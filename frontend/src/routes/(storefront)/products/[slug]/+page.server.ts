import { getProducts, getProductImages, getProductVariants } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  // products API returns by ID; find by slug via listing
  // TODO: add GET /products/by-slug/:slug endpoint to Go API for efficiency
  const products = await getProducts(100, 0).catch(() => []);
  const product = products.find((p) => p.slug === params.slug);

  if (!product) throw error(404, 'Product not found');

  const [variants, images] = await Promise.all([
    getProductVariants(product.id).catch(() => []),
    getProductImages(product.id).catch(() => [])
  ]);

  return { product, variants, images };
};
