import { adminListAuditLog, type AuditList, type AuditFilters } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const filters: AuditFilters = {
    action: url.searchParams.get('action') ?? undefined,
    entity_type: url.searchParams.get('entity_type') ?? undefined,
    admin_user_id: url.searchParams.get('admin_user_id') ?? undefined,
    from: url.searchParams.get('from') ?? undefined,
    to: url.searchParams.get('to') ?? undefined,
    limit: Number(url.searchParams.get('limit') ?? 50),
    offset: Number(url.searchParams.get('offset') ?? 0)
  };
  const list = await adminListAuditLog(token, filters).catch(
    () => ({ items: [], total: 0 } as AuditList)
  );
  return { list, filters };
};
