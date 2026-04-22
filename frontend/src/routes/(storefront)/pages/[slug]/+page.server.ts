import { getCmsPageBySlug } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  const page = await getCmsPageBySlug(params.slug).catch(() => null);
  if (!page) throw error(404, 'Page not found');
  return { page };
};
