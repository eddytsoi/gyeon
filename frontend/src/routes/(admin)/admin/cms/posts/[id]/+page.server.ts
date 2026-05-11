import {
  adminGetPost,
  adminCreatePost,
  adminUpdatePost,
  adminGetPostCategories,
  type CmsPost,
  type PostCategory
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const resolve = (token: string, id: string) =>
  id === 'new' ? Promise.resolve(id) : resolveAdminId(token, 'POST', id, '/admin/cms/posts');

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const id = await resolve(token, params.id);
  const [post, categories] = await Promise.all([
    id === 'new'
      ? Promise.resolve(null as CmsPost | null)
      : adminGetPost(token, id).catch(() => null),
    adminGetPostCategories(token).catch(() => [] as PostCategory[])
  ]);
  return { post, categories };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const id = await resolve(token, params.id);
    const data = await request.formData();

    const body = {
      slug: (data.get('slug') as string).trim(),
      title: (data.get('title') as string).trim(),
      excerpt: (data.get('excerpt') as string) || undefined,
      content: (data.get('content') as string) ?? '',
      cover_image_url: (data.get('cover_image_url') as string) || undefined,
      category_id: (data.get('category_id') as string) || undefined,
      category_ids: data.getAll('category_ids').map((v) => v.toString()).filter(Boolean),
      is_published: data.get('is_published') === 'true'
    };

    try {
      if (id === 'new') {
        await adminCreatePost(token, body);
      } else {
        await adminUpdatePost(token, id, body as CmsPost & { is_published: boolean });
      }
    } catch {
      return fail(500, { error: 'Failed to save post' });
    }

    redirect(303, '/admin/cms/posts');
  }
};
