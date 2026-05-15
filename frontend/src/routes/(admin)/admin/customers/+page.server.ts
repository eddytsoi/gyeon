import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { adminGetCustomers } from '$lib/api/admin';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const q = url.searchParams.get('q') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const res = await adminGetCustomers(token, PAGE_SIZE, offset, q).catch(() => ({ items: [], total: 0 }));

  return {
    customers: res.items,
    total: res.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    q
  };
};
