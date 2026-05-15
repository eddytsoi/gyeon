import { adminGetOrders, adminGetOrderNoticeUnreadCounts } from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const [ordersRes, unreadCounts] = await Promise.all([
    adminGetOrders(token, PAGE_SIZE, offset).catch(() => ({ items: [], total: 0 })),
    adminGetOrderNoticeUnreadCounts(token).catch(() => ({} as Record<string, number>))
  ]);
  return {
    orders: ordersRes.items,
    total: ordersRes.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    unreadCounts
  };
};
