import { adminGetOrders, adminGetOrderNoticeUnreadCounts, adminGetOrderCarriers } from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const PAGE_SIZE = 50;

const KNOWN_STATUSES = new Set([
  'pending', 'paid', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded'
]);

const KNOWN_ROLES = new Set(['customer', 'installer']);

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const q = url.searchParams.get('q') ?? '';

  const statuses = (url.searchParams.get('status') ?? '')
    .split(',')
    .map(s => s.trim())
    .filter(s => KNOWN_STATUSES.has(s));

  const fromParam = url.searchParams.get('from') ?? '';
  const toParam = url.searchParams.get('to') ?? '';
  const from = DATE_RE.test(fromParam) ? fromParam : '';
  const to = DATE_RE.test(toParam) ? toParam : '';

  const unreadParam = url.searchParams.get('unread') ?? '';
  const unread = unreadParam === '1' || unreadParam === 'true';

  const roles = (url.searchParams.get('role') ?? '')
    .split(',')
    .map(s => s.trim())
    .filter(s => KNOWN_ROLES.has(s));

  const hasNotesParam = url.searchParams.get('has_notes') ?? '';
  const hasNotes = hasNotesParam === '1' || hasNotesParam === 'true';

  const [ordersRes, unreadCounts] = await Promise.all([
    adminGetOrders(token, {
      limit: PAGE_SIZE,
      offset,
      q,
      statuses,
      from,
      to,
      unread,
      roles,
      hasNotes
    }).catch(() => ({ items: [], total: 0 })),
    adminGetOrderNoticeUnreadCounts(token).catch(() => ({} as Record<string, number>))
  ]);
  return {
    orders: ordersRes.items,
    total: ordersRes.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    unreadCounts,
    q,
    statuses,
    from,
    to,
    unread,
    roles,
    hasNotes
  };
};
