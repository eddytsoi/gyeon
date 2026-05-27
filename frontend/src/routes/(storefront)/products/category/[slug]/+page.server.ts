import { getCategoryBySlug, getProductsListPage } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const INITIAL_LIMIT = 12;

export const load: PageServerLoad = async ({ params, cookies }) => {
  // Forward the visitor's customer_token so the backend's per-role category
  // rules apply (installer sees installer-allowed categories; anonymous /
  // retail customers see the customer-allowed subset). Without this, every
  // SSR request looked anonymous to the backend and got the customer-role
  // filter regardless of who was actually logged in.
  const token = cookies.get('customer_token') ?? null;
  const category = await getCategoryBySlug(params.slug, token).catch(() => null);
  if (!category) throw error(404, 'Category not found');

  const page = await getProductsListPage(
    { limit: INITIAL_LIMIT, offset: 0, category: params.slug },
    undefined,
    token
  ).catch(() => ({ items: [], total: 0 }));

  return { category, products: page.items, total: page.total };
};
