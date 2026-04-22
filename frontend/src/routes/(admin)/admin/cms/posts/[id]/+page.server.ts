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

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [post, categories] = await Promise.all([
    params.id === 'new'
      ? Promise.resolve(null as CmsPost | null)
      : adminGetPost(token, params.id).catch(() => null),
    adminGetPostCategories(token).catch(() => [] as PostCategory[])
  ]);
  return { post, categories };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const body = {
      slug: (data.get('slug') as string).trim(),
      title: (data.get('title') as string).trim(),
      excerpt: (data.get('excerpt') as string) || undefined,
      content: (data.get('content') as string) ?? '',
      cover_image_url: (data.get('cover_image_url') as string) || undefined,
      category_id: (data.get('category_id') as string) || undefined,
      is_published: data.get('is_published') === 'true'
    };

    try {
      if (params.id === 'new') {
        await adminCreatePost(token, body);
      } else {
        await adminUpdatePost(token, params.id, body as CmsPost & { is_published: boolean });
      }
    } catch {
      return fail(500, { error: 'Failed to save post' });
    }

    redirect(303, '/admin/cms/posts');
  }
};
