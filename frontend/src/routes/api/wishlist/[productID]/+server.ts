import type { RequestHandler } from './$types';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const DELETE: RequestHandler = async ({ cookies, params }) => {
  const token = cookies.get('customer_token');
  if (!token) return new Response('Unauthorized', { status: 401 });
  const res = await fetch(`${API_BASE}/wishlist/${encodeURIComponent(params.productID)}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  });
  return new Response(null, { status: res.status });
};
