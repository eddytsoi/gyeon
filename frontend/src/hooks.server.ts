import type { Handle } from '@sveltejs/kit';
import { redirect } from '@sveltejs/kit';
import { sequence } from '@sveltejs/kit/hooks';
import { paraglideMiddleware } from '$lib/paraglide/server.js';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';
const MAINTENANCE_PATH = '/maintenance';

async function isMaintenanceMode(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/settings`, { signal: AbortSignal.timeout(2000) });
    if (!res.ok) return false;
    const settings: { key: string; value: string }[] = await res.json();
    return settings.find((s) => s.key === 'maintenance_mode')?.value === 'true';
  } catch {
    return false;
  }
}

const handleParaglide: Handle = ({ event, resolve }) =>
  paraglideMiddleware(event.request, ({ request, locale }) => {
    event.request = request;
    return resolve(event, {
      transformPageChunk: ({ html }) => html.replace('%paraglide.lang%', locale)
    });
  });

const handleMaintenance: Handle = async ({ event, resolve }) => {
  const { pathname } = event.url;

  // Always allow:
  //   • the maintenance page itself
  //   • admin routes, MCP, well-known discovery
  //   • /pay/ magic-link (complete payment for pending orders)
  //   • /checkout/success (post-payment confirmation page)
  //   • /account/* (login, register, setup-password, profile, order history)
  // Maintenance mode blocks NEW purchases (/checkout, /products, /cart) but
  // must not break customers who are mid-flow on an order they already placed.
  if (
    pathname === MAINTENANCE_PATH ||
    pathname.startsWith('/admin') ||
    pathname.startsWith('/mcp') ||
    pathname.startsWith('/.well-known') ||
    pathname.startsWith('/pay/') ||
    pathname.startsWith('/checkout/success') ||
    pathname.startsWith('/account/')
  ) return resolve(event);

  const adminToken = event.cookies.get('admin_token');

  // Logged-in admins bypass maintenance mode on all pages
  if (adminToken) return resolve(event);

  if (await isMaintenanceMode()) {
    throw redirect(302, MAINTENANCE_PATH);
  }

  return resolve(event);
};

export const handle: Handle = sequence(handleParaglide, handleMaintenance);
