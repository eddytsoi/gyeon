import { adminListSmtpLog, type SmtpLogList, type SmtpLogFilters } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ url, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const filters: SmtpLogFilters = {
    status: url.searchParams.get('status') ?? undefined,
    template_key: url.searchParams.get('template_key') ?? undefined,
    trigger_condition: url.searchParams.get('trigger_condition') ?? undefined,
    recipient: url.searchParams.get('recipient') ?? undefined,
    from: url.searchParams.get('from') ?? undefined,
    to: url.searchParams.get('to') ?? undefined,
    limit: PAGE_SIZE,
    offset
  };
  const list = await adminListSmtpLog(token, filters).catch(
    () => ({ items: [], total: 0 } as SmtpLogList)
  );
  return { list, filters, page: pageNum, pageSize: PAGE_SIZE };
};
