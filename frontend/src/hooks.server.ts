import type { Handle, RequestEvent } from '@sveltejs/kit';
import { redirect } from '@sveltejs/kit';
import { sequence } from '@sveltejs/kit/hooks';
import { paraglideMiddleware } from '$lib/paraglide/server.js';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';
const MAINTENANCE_PATH = '/maintenance';

const SUPPORTED_LOCALES = new Set(['en', 'zh-Hant']);
const PARAGLIDE_COOKIE = 'PARAGLIDE_LOCALE';

type PublicSettings = { maintenance: boolean; siteLocale: string };

// Per-request memo so handleParaglide and handleMaintenance share one fetch
// without holding stale data across requests. Stored on event.locals.
type LocalsWithSettings = App.Locals & { _publicSettings?: PublicSettings };

async function fetchPublicSettings(event: RequestEvent): Promise<PublicSettings> {
  const locals = event.locals as LocalsWithSettings;
  if (locals._publicSettings) return locals._publicSettings;

  let value: PublicSettings = { maintenance: false, siteLocale: 'en' };
  try {
    const res = await fetch(`${API_BASE}/settings`, { signal: AbortSignal.timeout(2000) });
    if (res.ok) {
      const settings: { key: string; value: string }[] = await res.json();
      const maintenance = settings.find((s) => s.key === 'maintenance_mode')?.value === 'true';
      const rawLocale = settings.find((s) => s.key === 'site_locale')?.value;
      const siteLocale = rawLocale && SUPPORTED_LOCALES.has(rawLocale) ? rawLocale : 'en';
      value = { maintenance, siteLocale };
    }
  } catch {
    /* keep defaults */
  }
  locals._publicSettings = value;
  return value;
}

const handleParaglide: Handle = async ({ event, resolve }) => {
  // Inject the admin-configured site_locale into the request cookie header
  // before paraglide's `cookie` strategy reads it. We always override any
  // existing PARAGLIDE_LOCALE cookie because that cookie is auto-written by
  // paraglide on first visit (from Accept-Language) — it does not represent
  // an explicit user choice. There is no language switcher in the storefront,
  // so the admin setting is the sole source of truth.
  const { siteLocale } = await fetchPublicSettings(event);
  if (siteLocale && SUPPORTED_LOCALES.has(siteLocale)) {
    const headers = new Headers(event.request.headers);
    const existing = headers.get('cookie') ?? '';
    const stripped = existing
      .split(/;\s*/)
      .filter((c) => c && !c.startsWith(`${PARAGLIDE_COOKIE}=`))
      .join('; ');
    const injected = stripped
      ? `${stripped}; ${PARAGLIDE_COOKIE}=${siteLocale}`
      : `${PARAGLIDE_COOKIE}=${siteLocale}`;
    headers.set('cookie', injected);
    event.request = new Request(event.request, { headers });
  }

  return paraglideMiddleware(event.request, ({ request, locale }) => {
    event.request = request;
    return resolve(event, {
      transformPageChunk: ({ html }) => html.replace('%paraglide.lang%', locale)
    });
  });
};

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

  const { maintenance } = await fetchPublicSettings(event);
  if (maintenance) {
    throw redirect(302, MAINTENANCE_PATH);
  }

  return resolve(event);
};

export const handle: Handle = sequence(handleParaglide, handleMaintenance);
