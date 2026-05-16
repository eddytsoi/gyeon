import { adminListQueueJobs, type QueueJobList, type QueueJobFilters } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ url, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const filters: QueueJobFilters = {
    status: url.searchParams.get('status') ?? undefined,
    type: url.searchParams.get('type') ?? undefined,
    from: url.searchParams.get('from') ?? undefined,
    to: url.searchParams.get('to') ?? undefined,
    limit: PAGE_SIZE,
    offset
  };
  const list = await adminListQueueJobs(token, filters).catch(
    () => ({ items: [], total: 0 } as QueueJobList)
  );
  return { list, filters, page: pageNum, pageSize: PAGE_SIZE };
};
