import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveCustomerOrderId } from '$lib/storefront/resolveOrderId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Customer-facing proxy. Same pattern as the admin one: reads the customer
// JWT from the HttpOnly cookie, forwards to the Go API, and pipes the PDF
// straight back to the browser. Order ownership is enforced on the API.
export const GET: RequestHandler = async ({ params, cookies, url, fetch }) => {
  const token = cookies.get('customer_token');
  if (!token) throw redirect(303, '/account/login');

  const id = await resolveCustomerOrderId(token, params.id);
  const locale =
    url.searchParams.get('locale') ?? cookies.get('PARAGLIDE_LOCALE') ?? 'en';

  const upstream = `${API_BASE}/customer-orders/${encodeURIComponent(id)}/receipt.pdf?locale=${encodeURIComponent(locale)}`;
  const res = await fetch(upstream, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw error(res.status, text || 'Failed to download receipt');
  }
  return new Response(res.body, {
    status: 200,
    headers: {
      'Content-Type': res.headers.get('content-type') ?? 'application/pdf',
      'Content-Disposition':
        res.headers.get('content-disposition') ?? 'attachment; filename="receipt.pdf"',
      'Cache-Control': 'private, no-store, max-age=0'
    }
  });
};
