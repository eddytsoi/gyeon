import { getBlogPostBySlug } from '$lib/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  const post = await getBlogPostBySlug(params.slug).catch(() => null);
  if (!post) throw error(404, 'Post not found');
  return { post };
};
