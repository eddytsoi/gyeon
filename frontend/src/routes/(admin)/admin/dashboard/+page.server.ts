import {
  getStats,
  adminGetRevenueTrend,
  adminGetTopProducts,
  adminGetTopCustomers,
  adminGetOrderStatusBreakdown,
  adminGetRefundTotal,
  type RevenuePoint,
  type TopProduct,
  type TopCustomer,
  type StatusBreakdownPoint
} from '$lib/api/admin';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) return { stats: null };

  const range = url.searchParams.get('range') ?? '30';
  const days = ['7', '30', '90'].includes(range) ? Number(range) : 30;
  const to = new Date();
  const from = new Date(to.getTime() - (days - 1) * 86_400_000);
  const fromISO = from.toISOString().slice(0, 10);
  const toISO = to.toISOString().slice(0, 10);
  const sortBy = (url.searchParams.get('by') === 'revenue' ? 'revenue' : 'qty') as 'qty' | 'revenue';

  const [stats, revenue, topProducts, topCustomers, statusBreakdown, refunds] = await Promise.all([
    getStats(token).catch(() => null),
    adminGetRevenueTrend(token, fromISO, toISO).catch(() => [] as RevenuePoint[]),
    adminGetTopProducts(token, fromISO, toISO, sortBy).catch(() => [] as TopProduct[]),
    adminGetTopCustomers(token, fromISO, toISO).catch(() => [] as TopCustomer[]),
    adminGetOrderStatusBreakdown(token, fromISO, toISO).catch(
      () => [] as StatusBreakdownPoint[]
    ),
    adminGetRefundTotal(token, fromISO, toISO).catch(() => ({ refunds: 0 }))
  ]);

  return {
    stats,
    range: String(days),
    sortBy,
    fromISO,
    toISO,
    revenue,
    topProducts,
    topCustomers,
    statusBreakdown,
    refundTotal: refunds.refunds
  };
};
