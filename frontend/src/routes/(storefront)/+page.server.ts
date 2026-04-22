import { getProducts, getProductImages, getProductVariants } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async () => {
  const products = (await getProducts(8, 0).catch(() => [])) ?? [];

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

  return { products: enriched };
};
