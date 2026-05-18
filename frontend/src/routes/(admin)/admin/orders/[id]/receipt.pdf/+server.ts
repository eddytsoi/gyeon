import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Server-side proxy so the browser never sees the admin JWT. Forwards the
// request to the Go API with the admin_token cookie added as Bearer auth,
// then streams the PDF response straight back to the browser.
export const GET: RequestHandler = async ({ params, cookies, url, fetch }) => {
  const token = cookies.get('admin_token');
  if (!token) throw redirect(303, '/admin/login');

  const id = await resolveAdminId(token, 'ORD', params.id, '/admin/orders');
  const locale =
    url.searchParams.get('locale') ?? cookies.get('PARAGLIDE_LOCALE') ?? 'en';

  const upstream = `${API_BASE}/admin/order-receipts/${encodeURIComponent(id)}/receipt.pdf?locale=${encodeURIComponent(locale)}`;
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
