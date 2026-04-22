import { getCategories, getProducts, getProductImages, getProductVariants } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
  const limit = 20;
  const offset = Number(url.searchParams.get('offset') ?? 0);

  const [products, categories] = await Promise.all([
    getProducts(limit, offset).catch(() => []).then(r => r ?? []),
    getCategories().catch(() => []).then(r => r ?? [])
  ]);

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

  return { products: enriched, categories, offset, limit };
};
