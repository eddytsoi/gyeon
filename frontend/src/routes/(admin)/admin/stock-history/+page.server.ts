import {
  adminListStockHistory,
  type StockMovementList,
  type StockMovementFilters
} from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ url, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageParam = Math.max(1, Number(url.searchParams.get('page') ?? 1));
  const sourceRaw = url.searchParams.get('source') ?? '';
  const source = sourceRaw === 'admin' || sourceRaw === 'order' ? sourceRaw : undefined;

  const filters: StockMovementFilters = {
    from: url.searchParams.get('from') ?? undefined,
    to: url.searchParams.get('to') ?? undefined,
    source,
    q: url.searchParams.get('q') ?? undefined,
    actor_user_id: url.searchParams.get('actor_user_id') ?? undefined,
    limit: PAGE_SIZE,
    offset: (pageParam - 1) * PAGE_SIZE
  };

  const list = await adminListStockHistory(token, filters).catch(
    () => ({ items: [], total: 0 } as StockMovementList)
  );

  return { list, filters, pageSize: PAGE_SIZE, currentPage: pageParam };
};
