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

    // Persist the locale on the response so the browser stores the cookie and
    // the client-side paraglide runtime resolves the same locale on hydration.
    // Without this, paraglide's cookie strategy sees the request cookie we
    // injected and skips writing Set-Cookie — the browser never gets it, and
    // the client falls through to preferredLanguage/baseLocale and re-renders
    // in a different locale (the "Chinese flashes then flips to English" bug).
    event.cookies.set(PARAGLIDE_COOKIE, siteLocale, {
      path: '/',
      sameSite: 'lax',
      maxAge: 60 * 60 * 24 * 365,
      httpOnly: false
    });
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

// In-process LRU-ish cache for redirect lookups. Short TTL keeps admin edits
// visible within ~60s without needing cache invalidation. Map preserves
// insertion order so we can evict oldest entries when over the cap.
type RedirectHit = { to: string; code: 301 | 302 } | null;
type RedirectCacheEntry = { value: RedirectHit; until: number };
const REDIRECT_CACHE = new Map<string, RedirectCacheEntry>();
const REDIRECT_CACHE_MAX = 500;
const REDIRECT_CACHE_TTL_MS = 60_000;

function cacheGet(path: string): RedirectHit | undefined {
  const entry = REDIRECT_CACHE.get(path);
  if (!entry) return undefined;
  if (entry.until < Date.now()) {
    REDIRECT_CACHE.delete(path);
    return undefined;
  }
  return entry.value;
}

function cacheSet(path: string, value: RedirectHit) {
  if (REDIRECT_CACHE.size >= REDIRECT_CACHE_MAX) {
    const oldest = REDIRECT_CACHE.keys().next().value;
    if (oldest) REDIRECT_CACHE.delete(oldest);
  }
  REDIRECT_CACHE.set(path, { value, until: Date.now() + REDIRECT_CACHE_TTL_MS });
}

const handleRedirect: Handle = async ({ event, resolve }) => {
  const { pathname } = event.url;

  // Only intercept storefront GETs. Admin routes, API proxies, MCP, and
  // SvelteKit internals must not go through redirect lookup.
  if (event.request.method !== 'GET') return resolve(event);
  if (
    pathname.startsWith('/admin') ||
    pathname.startsWith('/api') ||
    pathname.startsWith('/mcp') ||
    pathname.startsWith('/.well-known') ||
    pathname.startsWith('/_app')
  ) {
    return resolve(event);
  }

  let hit = cacheGet(pathname);
  if (hit === undefined) {
    try {
      const url = `${API_BASE}/redirects/match?path=${encodeURIComponent(pathname)}`;
      const res = await fetch(url, { signal: AbortSignal.timeout(1500) });
      if (res.ok) {
        const body = (await res.json()) as { to: string; code: number };
        const code = body.code === 302 ? 302 : 301;
        hit = { to: body.to, code };
      } else {
        hit = null;
      }
    } catch {
      hit = null; // fail-open: never block traffic on a redirect lookup
    }
    cacheSet(pathname, hit);
  }

  if (hit) {
    throw redirect(hit.code, hit.to);
  }
  return resolve(event);
};

// Content-Security-Policy: tuned to fit the third-party islands actually in
// use — GTM/Meta tracker bootstrap, reCAPTCHA v3, Stripe Elements. 'unsafe-
// inline' on script/style covers SvelteKit's hydration script and inline
// theme tokens; tightening to nonces is a follow-up.
const CSP_DIRECTIVES = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://www.googletagmanager.com https://connect.facebook.net https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/ https://js.stripe.com https://static.cloudflareinsights.com",
  "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
  "img-src 'self' data: blob: https:",
  "font-src 'self' data: https://fonts.gstatic.com",
  "connect-src 'self' https://www.google-analytics.com https://api.stripe.com https://www.googletagmanager.com https://www.facebook.com https://cloudflareinsights.com",
  "frame-src https://js.stripe.com https://www.google.com https://hooks.stripe.com",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'"
].join('; ');

const handleSecurityHeaders: Handle = async ({ event, resolve }) => {
  const response = await resolve(event);
  // Apply only to HTML responses — static assets carry their own headers and
  // don't benefit from CSP. SvelteKit serves the app as text/html.
  const ct = response.headers.get('content-type') ?? '';
  if (ct.includes('text/html')) {
    response.headers.set('Content-Security-Policy', CSP_DIRECTIVES);
  }
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  response.headers.set('X-Frame-Options', 'DENY');
  // Browsers ignore HSTS on plain-HTTP responses; behind Cloudflare/TLS this
  // pins the cert to a 6-month window with includeSubDomains.
  response.headers.set('Strict-Transport-Security', 'max-age=15552000; includeSubDomains');
  response.headers.set('Permissions-Policy', 'geolocation=(), microphone=(), camera=()');
  return response;
};

export const handle: Handle = sequence(handleParaglide, handleMaintenance, handleRedirect, handleSecurityHeaders);
