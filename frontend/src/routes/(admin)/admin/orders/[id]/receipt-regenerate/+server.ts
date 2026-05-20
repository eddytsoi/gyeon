import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Admin-triggered regenerate. Backend clears any cached PDF for this order
// and enqueues a fresh generate_receipt_cache job. Returns 202 from the API;
// SSE event drives the UI back to "ready" once the worker finishes.
export const POST: RequestHandler = async ({ params, cookies, url, fetch }) => {
  const token = cookies.get('admin_token');
  if (!token) throw redirect(303, '/admin/login');

  const id = await resolveAdminId(token, 'ORD', params.id, '/admin/orders');
  const locale =
    url.searchParams.get('locale') ?? cookies.get('PARAGLIDE_LOCALE') ?? 'zh-Hant';

  const upstream = `${API_BASE}/admin/order-receipts/${encodeURIComponent(id)}/receipt/regenerate?locale=${encodeURIComponent(locale)}`;
  const res = await fetch(upstream, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw error(res.status, text || 'Failed to enqueue receipt regenerate');
  }
  return new Response(null, { status: 202 });
};
