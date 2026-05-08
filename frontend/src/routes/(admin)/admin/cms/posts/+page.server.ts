import { adminGetPosts, adminGetPostCategories, adminDeletePost, type CmsPost, type PostCategory } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const q = url.searchParams.get('q') ?? '';
  const category = url.searchParams.get('category') ?? '';
  const [posts, categories] = await Promise.all([
    adminGetPosts(token, 50, 0, q, category).catch(() => [] as CmsPost[]),
    adminGetPostCategories(token).catch(() => [] as PostCategory[])
  ]);
  return { posts, categories, q, category };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      await adminDeletePost(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete post' });
    }
    redirect(303, '/admin/cms/posts');
  }
};
