import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { getPublicSettings } from '$lib/api';

export type AdminRole = 'super_admin' | 'admin' | 'editor';

// Path prefixes that require at least `admin` (block editor).
// Note: `/admin/discounts` is the sidebar slug for backend `/admin/pricing`.
const ADMIN_ONLY_PREFIXES = [
  '/admin/stock-mutations',
  '/admin/settings',
  '/admin/customers',
  '/admin/orders',
  '/admin/discounts',
  '/admin/audit-log',
  '/admin/redirects',
  '/admin/email-templates',
  '/admin/import',
  '/admin/smtp-log',
  '/admin/queue-jobs',
  '/admin/abandoned-cart'
];

// Path prefixes that require super_admin.
const SUPER_ADMIN_ONLY_PREFIXES = ['/admin/users'];

function decodeRole(token: string | undefined): AdminRole | null {
  if (!token) return null;
  try {
    const parts = token.split('.');
    if (parts.length < 2) return null;
    // base64url → base64
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const payload = JSON.parse(Buffer.from(b64, 'base64').toString('utf8'));
    const role = payload?.role;
    if (role === 'super_admin' || role === 'admin' || role === 'editor') return role;
    return null;
  } catch {
    return null;
  }
}

function pathMatchesPrefix(pathname: string, prefixes: string[]): boolean {
  return prefixes.some((p) => pathname === p || pathname.startsWith(p + '/'));
}

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token');
  const isLoginPage = url.pathname === '/admin/login';

  if (!token && !isLoginPage) throw redirect(303, '/admin/login');
  if (token && isLoginPage) throw redirect(303, '/admin/dashboard');

  const role = decodeRole(token);

  // Role-based redirect gate for direct URL access. Backend still enforces
  // 403, but redirecting in the layout avoids ugly blank pages on routes
  // the user has no business hitting.
  if (token && !isLoginPage) {
    if (role !== 'super_admin' && pathMatchesPrefix(url.pathname, SUPER_ADMIN_ONLY_PREFIXES)) {
      throw redirect(303, '/admin/dashboard');
    }
    if (role === 'editor' && pathMatchesPrefix(url.pathname, ADMIN_ONLY_PREFIXES)) {
      throw redirect(303, '/admin/dashboard');
    }
  }

  const publicSettings = await getPublicSettings().catch(() => []);
  const faviconUrl = publicSettings.find((s) => s.key === 'favicon_url')?.value ?? '';

  return { token: token ?? null, faviconUrl, role };
};
