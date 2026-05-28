import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

// Server-side proxy so the browser never sees the admin JWT. An anchor
// download can't set an Authorization header, so we read the admin_token
// cookie here and forward it as Bearer auth, then stream the ZIP straight
// back to the browser.
export const GET: RequestHandler = async ({ params, cookies, fetch }) => {
  const token = cookies.get('admin_token');
  if (!token) throw redirect(303, '/admin/login');

  const upstream = `${API_BASE}/admin/order-receipts/batch/${encodeURIComponent(params.batchId)}/download`;
  const res = await fetch(upstream, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw error(res.status, text || 'Failed to download receipts');
  }
  return new Response(res.body, {
    status: 200,
    headers: {
      'Content-Type': res.headers.get('content-type') ?? 'application/zip',
      'Content-Disposition':
        res.headers.get('content-disposition') ?? 'attachment; filename="receipts.zip"',
      'Cache-Control': 'private, no-store, max-age=0'
    }
  });
};
