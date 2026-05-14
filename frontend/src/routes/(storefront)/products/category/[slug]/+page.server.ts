import { getCategoryBySlug, getProductsListPage } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const INITIAL_LIMIT = 12;

export const load: PageServerLoad = async ({ params }) => {
  const category = await getCategoryBySlug(params.slug).catch(() => null);
  if (!category) throw error(404, 'Category not found');

  const page = await getProductsListPage({
    limit: INITIAL_LIMIT,
    offset: 0,
    category: params.slug
  }).catch(() => ({ items: [], total: 0 }));

  return { category, products: page.items, total: page.total };
};
