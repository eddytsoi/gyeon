import {
  getCategoryBySlug,
  getProductsByCategorySlug,
  getProductImages,
  getProductVariants
} from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, url }) => {
  const limit = 20;
  const offset = Number(url.searchParams.get('offset') ?? 0);

  const category = await getCategoryBySlug(params.slug).catch(() => null);
  if (!category) throw error(404, 'Category not found');

  const products = await getProductsByCategorySlug(params.slug, limit, offset)
    .catch(() => [])
    .then((r) => r ?? []);

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

  return { category, products: enriched, offset, limit };
};
