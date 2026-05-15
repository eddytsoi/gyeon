import { adminListForms, adminDeleteForm } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const res = await adminListForms(token, PAGE_SIZE, offset).catch(() => ({ items: [], total: 0 }));
  return {
    forms: res.items,
    total: res.total,
    page: pageNum,
    pageSize: PAGE_SIZE
  };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      await adminDeleteForm(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete form' });
    }
    redirect(303, '/admin/forms');
  }
};
