import { getMyOrders, getMyOrderNoticeUnreadCounts } from '$lib/api';
import type { PageServerLoad } from './$types';

const LIMIT = 20;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  const offset = Math.max(0, Number(url.searchParams.get('offset') ?? 0) || 0);
  const status = url.searchParams.get('status') ?? '';
  const q = url.searchParams.get('q') ?? '';
  const [result, unreadCounts] = await Promise.all([
    token
      ? getMyOrders(token, LIMIT, offset, status, q).catch(() => ({ orders: [], total: 0 }))
      : Promise.resolve({ orders: [], total: 0 }),
    token ? getMyOrderNoticeUnreadCounts(token).catch(() => ({} as Record<string, number>)) : Promise.resolve({} as Record<string, number>)
  ]);
  return { orders: result.orders, total: result.total, offset, limit: LIMIT, status, q, unreadCounts };
};
