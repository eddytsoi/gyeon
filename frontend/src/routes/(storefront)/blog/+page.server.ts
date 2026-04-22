import { getBlogPosts } from '$lib/api';
import type { CmsPost } from '$lib/types';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async () => {
  const posts = await getBlogPosts(20, 0).catch(() => [] as CmsPost[]);
  return { posts };
};
