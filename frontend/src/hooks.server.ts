import type { Handle } from '@sveltejs/kit';
import { redirect } from '@sveltejs/kit';

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

export const handle: Handle = async ({ event, resolve }) => {
  const { pathname } = event.url;

  // Always allow the maintenance page, admin routes, MCP, and well-known discovery
  if (
    pathname === MAINTENANCE_PATH ||
    pathname.startsWith('/admin') ||
    pathname.startsWith('/mcp') ||
    pathname.startsWith('/.well-known')
  ) return resolve(event);

  const adminToken = event.cookies.get('admin_token');

  // Logged-in admins bypass maintenance mode on all pages
  if (adminToken) return resolve(event);

  if (await isMaintenanceMode()) {
    throw redirect(302, MAINTENANCE_PATH);
  }

  return resolve(event);
};
