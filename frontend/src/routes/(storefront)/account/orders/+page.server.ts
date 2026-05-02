import { getMyOrders, getMyOrderNoticeUnreadCounts } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  const offset = Number(url.searchParams.get('offset') ?? 0);
  const [orders, unreadCounts] = await Promise.all([
    token ? getMyOrders(token, 20, offset).catch(() => []) : Promise.resolve([]),
    token ? getMyOrderNoticeUnreadCounts(token).catch(() => ({} as Record<string, number>)) : Promise.resolve({} as Record<string, number>)
  ]);
  return { orders, offset, unreadCounts };
};
