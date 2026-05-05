// Storefront proxy for the customer wishlist endpoint. Reads the httpOnly
// `customer_token` cookie set at login and forwards it as a Bearer token, so
// the client never has to handle the JWT directly.
import type { RequestHandler } from './$types';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

function authHeaders(cookieToken: string | undefined): Record<string, string> | null {
  if (!cookieToken) return null;
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${cookieToken}`
  };
}

export const GET: RequestHandler = async ({ cookies }) => {
  const headers = authHeaders(cookies.get('customer_token'));
  if (!headers) return new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } });
  const res = await fetch(`${API_BASE}/wishlist/`, { headers });
  return new Response(await res.text(), {
    status: res.status,
    headers: { 'Content-Type': 'application/json' }
  });
};

export const POST: RequestHandler = async ({ cookies, request }) => {
  const headers = authHeaders(cookies.get('customer_token'));
  if (!headers) return new Response('Unauthorized', { status: 401 });
  const body = await request.text();
  const res = await fetch(`${API_BASE}/wishlist/`, { method: 'POST', headers, body });
  return new Response(await res.text(), {
    status: res.status,
    headers: { 'Content-Type': 'application/json' }
  });
};
