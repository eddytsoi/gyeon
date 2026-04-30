import { getBlogCategoryBySlug, getBlogPostsByCategorySlug } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { CmsPost } from '$lib/types';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  const category = await getBlogCategoryBySlug(params.slug).catch(() => null);
  if (!category) throw error(404, 'Category not found');

  const posts = await getBlogPostsByCategorySlug(params.slug, 20, 0).catch(() => [] as CmsPost[]);
  return { category, posts };
};
