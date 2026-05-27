// Proxy for the inventory-snapshot CSV. Reads the httpOnly admin_token
// cookie server-side, calls the backend, and streams the CSV to the client.
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const BACKEND = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const GET: RequestHandler = async ({ cookies, fetch }) => {
  const token = cookies.get('admin_token') ?? '';
  if (!token) throw error(401, 'not signed in');

  const upstream = await fetch(`${BACKEND}/admin/stock-mutations/inventory.csv`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!upstream.ok) throw error(upstream.status, 'failed to export inventory');

  const headers = new Headers();
  for (const k of ['content-type', 'content-length', 'content-disposition']) {
    const v = upstream.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new Response(upstream.body, { status: 200, headers });
};
