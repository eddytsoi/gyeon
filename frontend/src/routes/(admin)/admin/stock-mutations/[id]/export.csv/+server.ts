// Proxy for downloading a stock mutation as CSV. admin_token is httpOnly so
// the browser can't add it as a Bearer header — this handler reads the cookie
// server-side, calls the backend, and streams the CSV through to the client.
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const BACKEND = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const GET: RequestHandler = async ({ params, cookies, fetch }) => {
  const token = cookies.get('admin_token') ?? '';
  if (!token) throw error(401, 'not signed in');

  const upstream = await fetch(`${BACKEND}/admin/stock-mutations/${params.id}/export.csv`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (upstream.status === 404) throw error(404, 'mutation not found');
  if (!upstream.ok) throw error(upstream.status, 'failed to export');

  const headers = new Headers();
  for (const k of ['content-type', 'content-length', 'content-disposition']) {
    const v = upstream.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new Response(upstream.body, { status: 200, headers });
};
