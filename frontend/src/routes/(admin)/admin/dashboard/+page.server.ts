import {
  adminGetDashboard,
  adminGetDashboardPrefs,
  adminGetTopProducts,
  adminGetTopCustomers,
  adminGetOrderStatusBreakdown,
  adminGetRevenueBreakdown,
  adminGetLowStock,
  adminGetCategories,
  adminGetOrders,
  adminListShipanyCouriers,
  type DashFilters,
  type DashCompareMode,
  type TopProduct,
  type TopCustomer,
  type StatusBreakdownPoint,
  type RevenueBreakdownRow,
  type DashboardResponse,
  type DashboardPrefs
} from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const VALID_ROLES = ['customer', 'installer', 'installer_v2'];
const VALID_COMPARE = ['prev_month', 'prev_period', 'prev_year', 'none'];

function localISO(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();

  // Default range is the current month-to-date — the owner's most-used view, and
  // it lines the KPI cards up with the always-month-over-month hero band.
  const today = new Date();
  const fromISO = url.searchParams.get('from') || localISO(new Date(today.getFullYear(), today.getMonth(), 1));
  const toISO = url.searchParams.get('to') || localISO(today);

  const roles = (url.searchParams.get('role') ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter((r) => VALID_ROLES.includes(r));
  const category = url.searchParams.get('category') ?? '';
  const sortBy = (url.searchParams.get('by') === 'revenue' ? 'revenue' : 'qty') as 'qty' | 'revenue';

  // Filter-bar state is always returned so the bar renders even when data fails.
  const filterEcho = {
    fromISO,
    toISO,
    roles,
    category,
    sortBy,
    compare: 'prev_month' as DashCompareMode,
    prefs: null as DashboardPrefs | null,
    categories: [] as Awaited<ReturnType<typeof adminGetCategories>>
  };

  if (!token) return { dashboard: null, ...filterEcho };

  // Per-admin prefs first so the compare-mode default and saved layout are known
  // before the (heavier) consolidated metrics call — prefs is a tiny query.
  const prefs = await adminGetDashboardPrefs(token).catch(() => null);
  const urlCompare = url.searchParams.get('compare');
  const compare = (VALID_COMPARE.includes(urlCompare ?? '')
    ? urlCompare
    : prefs?.compare_mode && VALID_COMPARE.includes(prefs.compare_mode)
      ? prefs.compare_mode
      : 'prev_month') as DashCompareMode;

  const f: DashFilters = { from: fromISO, to: toISO, roles, category: category || undefined };

  const [
    dashboard,
    topProducts,
    topCustomers,
    statusBreakdown,
    byCategory,
    byRole,
    byCarrier,
    recentOrders,
    lowStock,
    categories,
    couriers
  ] = await Promise.all([
    adminGetDashboard(token, f, compare).catch(() => null as DashboardResponse | null),
    adminGetTopProducts(token, f, sortBy).catch(() => [] as TopProduct[]),
    adminGetTopCustomers(token, f).catch(() => [] as TopCustomer[]),
    adminGetOrderStatusBreakdown(token, f).catch(() => [] as StatusBreakdownPoint[]),
    adminGetRevenueBreakdown(token, 'category', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetRevenueBreakdown(token, 'role', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetRevenueBreakdown(token, 'carrier', f).catch(() => [] as RevenueBreakdownRow[]),
    adminGetOrders(token, { from: fromISO, to: toISO, roles, limit: 8 }).catch(() => ({ items: [], total: 0 })),
    adminGetLowStock(token).catch(() => []),
    adminGetCategories(token).catch(() => []),
    adminListShipanyCouriers(token).catch(() => [])
  ]);

  return {
    dashboard,
    prefs,
    compare,
    topProducts,
    topCustomers,
    statusBreakdown,
    byCategory,
    byRole,
    byCarrier,
    couriers,
    recentOrders: recentOrders.items ?? [],
    lowStock,
    categories,
    fromISO,
    toISO,
    roles,
    category,
    sortBy
  };
};
