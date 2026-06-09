import { adminGetStockVelocity, type StockVelocitySort } from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const WINDOWS = [7, 15, 30, 60, 90, 180, 365];
const DEFAULT_SORT: StockVelocitySort = 'gross_sold_desc';
const SORTS = new Set<StockVelocitySort>([
  'gross_sales_desc', 'gross_sales_asc',
  'gross_sold_desc', 'gross_sold_asc',
  'daily_gross_sold_desc', 'daily_gross_sold_asc',
  'days_left_asc', 'days_left_desc',
  'stock_desc', 'stock_asc',
  'variation_asc', 'variation_desc',
]);

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const d = parseInt(url.searchParams.get('days') ?? '30', 10);
  const days = WINDOWS.includes(d) ? d : 30;

  const sortParam = url.searchParams.get('sort') as StockVelocitySort | null;
  const sort: StockVelocitySort = sortParam && SORTS.has(sortParam) ? sortParam : DEFAULT_SORT;

  const res = await adminGetStockVelocity(token, { days, sort }).catch(() => ({
    items: [],
    total: 0,
    days,
    capped: false,
  }));

  return {
    rows: res.items,
    total: res.total,
    capped: res.capped,
    days,
    sort,
    windows: WINDOWS,
  };
};
