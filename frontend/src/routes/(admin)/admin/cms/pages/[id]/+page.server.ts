import {
  adminGetPage,
  adminCreatePage,
  adminUpdatePage,
  type CmsPage
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  if (params.id === 'new') {
    return { page: null as CmsPage | null };
  }
  const page = await adminGetPage(token, params.id).catch(() => null);
  return { page };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const body = {
      slug: (data.get('slug') as string).trim(),
      title: (data.get('title') as string).trim(),
      content: (data.get('content') as string) ?? '',
      meta_title: (data.get('meta_title') as string) || undefined,
      meta_desc: (data.get('meta_desc') as string) || undefined,
      is_published: data.get('is_published') === 'true'
    };

    try {
      if (params.id === 'new') {
        await adminCreatePage(token, body);
      } else {
        await adminUpdatePage(token, params.id, body as CmsPage & { is_published: boolean });
      }
    } catch (e) {
      return fail(500, { error: 'Failed to save page' });
    }

    redirect(303, '/admin/cms/pages');
  }
};
