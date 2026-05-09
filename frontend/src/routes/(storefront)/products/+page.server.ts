import { getCategories, getProductsListPage, type ProductListFilters } from '$lib/api';
import type { PageServerLoad } from './$types';

const SORT_VALUES = new Set(['new', 'price_asc', 'price_desc', 'name']);
const INITIAL_LIMIT = 12;

function parsePositiveFloat(s: string | null): number | undefined {
  if (!s) return undefined;
  const n = Number(s);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}

export const load: PageServerLoad = async ({ url }) => {
  const q = url.searchParams.get('q') ?? '';
  const category = url.searchParams.get('category') ?? '';
  const sortRaw = url.searchParams.get('sort') ?? '';
  const sort = (SORT_VALUES.has(sortRaw) ? sortRaw : 'new') as ProductListFilters['sort'];
  const minPrice = parsePositiveFloat(url.searchParams.get('min_price'));
  const maxPrice = parsePositiveFloat(url.searchParams.get('max_price'));

  const filters: ProductListFilters = {
    limit: INITIAL_LIMIT,
    offset: 0,
    search: q || undefined,
    category: category || undefined,
    minPrice,
    maxPrice,
    sort
  };

  const [page, categories] = await Promise.all([
    getProductsListPage(filters).catch(() => ({ items: [], total: 0 })),
    getCategories().catch(() => []).then(r => r ?? [])
  ]);

  return {
    products: page.items,
    total: page.total,
    categories,
    initialLimit: INITIAL_LIMIT,
    q,
    category,
    sort,
    minPrice: minPrice ?? null,
    maxPrice: maxPrice ?? null
  };
};
