import {
  getStats,
  adminGetRevenueTrend,
  adminGetTopProducts,
  adminGetTopCustomers,
  adminGetOrderStatusBreakdown,
  adminGetRefundSummary,
  adminGetDashboardSummary,
  adminGetRevenueBreakdown,
  adminGetLowStock,
  adminGetCategories,
  adminGetOrders,
  type DashFilters,
  type RevenuePoint,
  type TopProduct,
  type TopCustomer,
  type StatusBreakdownPoint,
  type RevenueBreakdownRow,
  type RefundSummary,
  type DashboardSummary
} from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const VALID_ROLES = ['customer', 'installer', 'installer_v2'];

function localISO(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();

  // Date range — default to the last 30 days when not provided.
  const today = new Date();
  const fromISO = url.searchParams.get('from') || localISO(new Date(today.getTime() - 29 * 86_400_000));
  const toISO = url.searchParams.get('to') || localISO(today);

  const roles = (url.searchParams.get('role') ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter((r) => VALID_ROLES.includes(r));
  const category = url.searchParams.get('category') ?? '';
  const sortBy = (url.searchParams.get('by') === 'revenue' ? 'revenue' : 'qty') as 'qty' | 'revenue';

  // Filter-bar state is always returned so the bar renders even when stats fail.
  const filterEcho = { fromISO, toISO, roles, category, sortBy, categories: [] as Awaited<ReturnType<typeof adminGetCategories>> };

  if (!token) return { stats: null, ...filterEcho };

  const f: DashFilters = { from: fromISO, to: toISO, roles, category: category || undefined };

  const [
    stats,
    summary,
    revenue,
    topProducts,
    topCustomers,
    statusBreakdown,
    refund,
    byCategory,
    byRole,
    byCarrier,
    recentOrders,
    lowStock,
    categories
  ] = await Promise.all([
    getStats(token, f).catch(() => null),
    adminGetDashboardSummary(token, f).catch(() => null),
    adminGetRevenueTrend(token, f).catch(() => [] as RevenuePoint[]),
    adminGetTopProducts(token, f, sortBy).catch(() => [] as TopProduct[]),
    adminGetTopCustomers(token, f).catch(() => [] as TopCustomer[]),
    adminGetOrderStatusBreakdown(token, f).catch(() => [] as StatusBreakdownPoint[]),
    adminGetRefundSummary(token, f).catch(() => null as RefundSummary | null),
    adminGetRevenueBreakdown(token, 'category', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetRevenueBreakdown(token, 'role', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetRevenueBreakdown(token, 'carrier', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetOrders(token, { from: fromISO, to: toISO, roles, limit: 8 }).catch(() => ({ items: [], total: 0 })),
    adminGetLowStock(token).catch(() => []),
    adminGetCategories(token).catch(() => [])
  ]);

  return {
    stats,
    summary,
    revenue,
    topProducts,
    topCustomers,
    statusBreakdown,
    refund,
    refundTotal: refund?.refunds ?? 0,
    byCategory,
    byRole,
    byCarrier,
    recentOrders: recentOrders.items ?? [],
    lowStock,
    categories,
    // Echo the active filters so the filter bar can render its state.
    fromISO,
    toISO,
    roles,
    category,
    sortBy
  };
};
