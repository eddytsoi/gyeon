// Proxy for downloading the Stock Velocity report as CSV. admin_token is httpOnly
// so the browser can't add it as a Bearer header — this handler reads the cookie
// server-side, forwards the window + sort, and streams the CSV through.
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const BACKEND = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const GET: RequestHandler = async ({ url, cookies, fetch }) => {
  const token = cookies.get('admin_token') ?? '';
  if (!token) throw error(401, 'not signed in');

  const qs = new URLSearchParams();
  const days = url.searchParams.get('days');
  if (days) qs.set('days', days);
  const sort = url.searchParams.get('sort');
  if (sort) qs.set('sort', sort);

  const upstream = await fetch(`${BACKEND}/admin/stock-velocity/export.csv?${qs.toString()}`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!upstream.ok) throw error(upstream.status, 'failed to export');

  const headers = new Headers();
  for (const k of ['content-type', 'content-length', 'content-disposition']) {
    const v = upstream.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new Response(upstream.body, { status: 200, headers });
};
