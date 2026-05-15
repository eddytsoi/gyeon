import { adminListAuditLog, type AuditList, type AuditFilters } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ url, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const filters: AuditFilters = {
    action: url.searchParams.get('action') ?? undefined,
    entity_type: url.searchParams.get('entity_type') ?? undefined,
    admin_user_id: url.searchParams.get('admin_user_id') ?? undefined,
    from: url.searchParams.get('from') ?? undefined,
    to: url.searchParams.get('to') ?? undefined,
    limit: PAGE_SIZE,
    offset
  };
  const list = await adminListAuditLog(token, filters).catch(
    () => ({ items: [], total: 0 } as AuditList)
  );
  return { list, filters, page: pageNum, pageSize: PAGE_SIZE };
};
