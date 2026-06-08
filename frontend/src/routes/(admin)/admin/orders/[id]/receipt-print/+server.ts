import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Admin-triggered manual receipt reprint. Backend enqueues a print_receipt
// job (Force=true, bypassing the auto-enabled toggle) that POSTs the receipt
// PDF to PrintNode. Returns 202; the physical print happens off the request
// path in the queue worker.
export const POST: RequestHandler = async ({ params, cookies, fetch }) => {
  const token = cookies.get('admin_token');
  if (!token) throw redirect(303, '/admin/login');

  const id = await resolveAdminId(token, 'ORD', params.id, '/admin/orders');

  const upstream = `${API_BASE}/admin/printnode/orders/${encodeURIComponent(id)}/print`;
  const res = await fetch(upstream, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw error(res.status, text || 'Failed to enqueue receipt print');
  }
  return new Response(null, { status: 202 });
};
