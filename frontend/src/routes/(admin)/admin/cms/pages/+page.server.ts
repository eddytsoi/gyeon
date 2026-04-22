import { adminGetPages, adminDeletePage, type CmsPage } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const pages = await adminGetPages(token).catch(() => [] as CmsPage[]);
  return { pages };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      await adminDeletePage(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete page' });
    }
    redirect(303, '/admin/cms/pages');
  }
};
