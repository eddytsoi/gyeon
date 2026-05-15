import { getBlogPosts } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { CmsPost } from '$lib/types';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { blogEnabled } = await parent();
  if (!blogEnabled) throw error(404, 'Not found');
  const posts = await getBlogPosts(20, 0).catch(() => [] as CmsPost[]);
  return { posts };
};
