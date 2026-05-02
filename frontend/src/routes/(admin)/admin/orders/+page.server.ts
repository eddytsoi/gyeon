import { adminGetOrders, adminGetOrderNoticeUnreadCounts } from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  const [orders, unreadCounts] = await Promise.all([
    adminGetOrders(token).catch(() => []).then(r => r ?? []),
    adminGetOrderNoticeUnreadCounts(token).catch(() => ({} as Record<string, number>))
  ]);
  return { orders, unreadCounts };
};
