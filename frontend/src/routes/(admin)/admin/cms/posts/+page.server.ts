import { adminGetPosts, adminGetPostCategories, adminDeletePost, type PostCategory } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const q = url.searchParams.get('q') ?? '';
  const category = url.searchParams.get('category') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const [postsRes, categories] = await Promise.all([
    adminGetPosts(token, PAGE_SIZE, offset, q, category).catch(() => ({ items: [], total: 0 })),
    adminGetPostCategories(token).catch(() => [] as PostCategory[])
  ]);
  return {
    posts: postsRes.items,
    total: postsRes.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    categories,
    q,
    category,
  };
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
