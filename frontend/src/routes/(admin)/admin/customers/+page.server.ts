import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { adminGetCustomers } from '$lib/api/admin';
import type { CustomerRole } from '$lib/types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const q = url.searchParams.get('q') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const activeParam = url.searchParams.get('active') ?? '';
  const active = activeParam === 'active' || activeParam === 'inactive' ? activeParam : undefined;
  const roleParam = url.searchParams.get('role') ?? '';
  const role = roleParam === 'customer' || roleParam === 'installer' ? (roleParam as CustomerRole) : undefined;

  const res = await adminGetCustomers(token, PAGE_SIZE, offset, q, { active, role }).catch(() => ({ items: [], total: 0 }));

  return {
    customers: res.items,
    total: res.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    q,
    active: active ?? '',
    role: role ?? ''
  };
};
