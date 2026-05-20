import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// JSON proxy for the admin lightning-icon poll. Mirrors receipt.pdf/+server.ts
// in structure: forwards the admin_token cookie as a Bearer header so the
// browser never sees the JWT.
export const GET: RequestHandler = async ({ params, cookies, url, fetch }) => {
  const token = cookies.get('admin_token');
  if (!token) throw redirect(303, '/admin/login');

  const id = await resolveAdminId(token, 'ORD', params.id, '/admin/orders');
  const locale =
    url.searchParams.get('locale') ?? cookies.get('PARAGLIDE_LOCALE') ?? 'zh-Hant';

  const upstream = `${API_BASE}/admin/order-receipts/${encodeURIComponent(id)}/receipt-cache-status?locale=${encodeURIComponent(locale)}`;
  const res = await fetch(upstream, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw error(res.status, text || 'Failed to fetch receipt cache status');
  }
  return new Response(res.body, {
    status: 200,
    headers: {
      'Content-Type': res.headers.get('content-type') ?? 'application/json',
      'Cache-Control': 'private, no-store, max-age=0'
    }
  });
};
