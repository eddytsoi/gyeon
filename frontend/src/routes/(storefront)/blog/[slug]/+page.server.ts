import { getBlogPostBySlug } from '$lib/api';
import { error } from '@sveltejs/kit';
import { scanShortcodeRefs } from '$lib/shortcodes/scan';
import { resolveShortcodeRefs } from '$lib/shortcodes/resolve';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, parent }) => {
  const { blogEnabled } = await parent();
  if (!blogEnabled) throw error(404, 'Not found');
  const post = await getBlogPostBySlug(params.slug).catch(() => null);
  if (!post) throw error(404, 'Post not found');
  const shortcodeRefs = await resolveShortcodeRefs(scanShortcodeRefs(post.content));
  return { post, shortcodeRefs };
};
