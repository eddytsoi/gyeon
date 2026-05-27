import { getCategories, getProductsFiltered, getProductsListPage, type ProductListFilters } from '$lib/api';
import type { PageServerLoad } from './$types';

const SORT_VALUES = new Set(['new', 'price_asc', 'price_desc', 'name']);
const INITIAL_LIMIT = 12;
const PRICE_BOUND_FLOOR = 100;

function parsePositiveFloat(s: string | null): number | undefined {
  if (!s) return undefined;
  const n = Number(s);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}

export const load: PageServerLoad = async ({ url, cookies }) => {
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

  // Forward customer_token so the backend's per-role category rules apply.
  // Without this, an installer's first-paint product grid is filtered for
  // the anonymous-/customer-role view.
  const token = cookies.get('customer_token') ?? null;
  const [page, categories, costliest] = await Promise.all([
    getProductsListPage(filters, undefined, token).catch(() => ({ items: [], total: 0 })),
    getCategories(token).catch(() => []).then(r => r ?? []),
    // Single most-expensive product (unfiltered) → stable upper bound for the slider.
    getProductsFiltered({ limit: 1, offset: 0, sort: 'price_desc' }, undefined, token).catch(() => [])
  ]);

  const apiMax = costliest[0]?.min_price ?? 0;
  const priceMax = Math.max(PRICE_BOUND_FLOOR, Math.ceil(apiMax / 100) * 100);

  return {
    products: page.items,
    total: page.total,
    categories,
    initialLimit: INITIAL_LIMIT,
    q,
    category,
    sort,
    minPrice: minPrice ?? null,
    maxPrice: maxPrice ?? null,
    priceMax
  };
};
