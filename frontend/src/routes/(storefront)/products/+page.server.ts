import { getCategories, getProductsFiltered, getProductImages, getProductVariants, type ProductListFilters } from '$lib/api';
import type { PageServerLoad } from './$types';

const SORT_VALUES = new Set(['new', 'price_asc', 'price_desc', 'name']);

function parsePositiveFloat(s: string | null): number | undefined {
  if (!s) return undefined;
  const n = Number(s);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}

export const load: PageServerLoad = async ({ url }) => {
  const limit = 20;
  const offset = Number(url.searchParams.get('offset') ?? 0);
  const q = url.searchParams.get('q') ?? '';
  const category = url.searchParams.get('category') ?? '';
  const sortRaw = url.searchParams.get('sort') ?? '';
  const sort = (SORT_VALUES.has(sortRaw) ? sortRaw : 'new') as ProductListFilters['sort'];
  const minPrice = parsePositiveFloat(url.searchParams.get('min_price'));
  const maxPrice = parsePositiveFloat(url.searchParams.get('max_price'));

  const filters: ProductListFilters = {
    limit, offset,
    search: q || undefined,
    category: category || undefined,
    minPrice, maxPrice,
    sort
  };

  const [products, categories] = await Promise.all([
    getProductsFiltered(filters).catch(() => []).then(r => r ?? []),
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

  return {
    products: enriched,
    categories,
    offset, limit,
    q,
    category,
    sort,
    minPrice: minPrice ?? null,
    maxPrice: maxPrice ?? null
  };
};
