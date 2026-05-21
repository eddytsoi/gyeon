import {
  adminListStockMutations,
  type StockMutationFilters,
  type StockMutationList,
  type StockMutationStatus,
  type StockMutationType
} from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const statusRaw = url.searchParams.get('status') ?? '';
  const status: StockMutationStatus | '' =
    statusRaw === 'draft' || statusRaw === 'executed' ? statusRaw : '';

  const typeRaw = url.searchParams.get('type') ?? '';
  const type: StockMutationType | '' = typeRaw === 'in' || typeRaw === 'out' ? typeRaw : '';

  const fromParam = url.searchParams.get('from') ?? '';
  const toParam = url.searchParams.get('to') ?? '';
  const from = DATE_RE.test(fromParam) ? fromParam : '';
  const to = DATE_RE.test(toParam) ? toParam : '';
  const q = url.searchParams.get('q') ?? '';

  const filters: StockMutationFilters = {
    status,
    type,
    from: from || undefined,
    to: to || undefined,
    q: q || undefined,
    limit: PAGE_SIZE,
    offset
  };

  const list = await adminListStockMutations(token, filters).catch(
    () => ({ items: [], total: 0 } as StockMutationList)
  );

  return {
    list,
    page: pageNum,
    pageSize: PAGE_SIZE,
    q,
    status,
    type,
    from,
    to
  };
};
