import type { RequestHandler } from './$types';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const POST: RequestHandler = async ({ cookies, request }) => {
  const token = cookies.get('customer_token');
  if (!token) return new Response('Unauthorized', { status: 401 });
  const body = await request.text();
  const res = await fetch(`${API_BASE}/wishlist/merge`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`
    },
    body
  });
  return new Response(await res.text(), {
    status: res.status,
    headers: { 'Content-Type': 'application/json' }
  });
};
