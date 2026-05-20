import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveCustomerOrderId } from '$lib/storefront/resolveOrderId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Customer-facing JSON proxy used by the order detail page to poll for the
// lightning-icon state every few seconds while the queue worker is warming
// the cache. Once the icon lights up the page stops polling.
export const GET: RequestHandler = async ({ params, cookies, url, fetch }) => {
  const token = cookies.get('customer_token');
  if (!token) throw redirect(303, '/account/login');

  const id = await resolveCustomerOrderId(token, params.id);
  const locale =
    url.searchParams.get('locale') ?? cookies.get('PARAGLIDE_LOCALE') ?? 'zh-Hant';

  const upstream = `${API_BASE}/customer-orders/${encodeURIComponent(id)}/receipt-cache-status?locale=${encodeURIComponent(locale)}`;
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
