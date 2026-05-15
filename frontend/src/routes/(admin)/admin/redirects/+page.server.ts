import {
  adminListRedirects,
  adminDeleteRedirect
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const res = await adminListRedirects(token, PAGE_SIZE, offset).catch(() => ({ items: [], total: 0 }));
  return {
    items: res.items,
    total: res.total,
    page: pageNum,
    pageSize: PAGE_SIZE
  };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const id = (await request.formData()).get('id') as string;
    try {
      await adminDeleteRedirect(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete redirect' });
    }
    redirect(303, '/admin/redirects');
  }
};
