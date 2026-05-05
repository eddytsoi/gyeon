/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true" />
/// <reference lib="esnext" />
/// <reference lib="webworker" />

/*
 * P3 #25 — service worker for offline app shell.
 *
 * SvelteKit injects the build manifest at compile time via $service-worker.
 * `build` = hashed app assets (immutable, safe to long-cache).
 * `files` = static/ contents.
 * `prerendered` = unused here (no prerendered routes).
 *
 * Strategy:
 *   - Install: pre-cache the build + static manifest under ASSETS_CACHE.
 *   - Fetch: cache-first for those assets; pass through everything else
 *     (storefront pages, /api/*, /uploads/*) so the SW never serves stale
 *     dynamic content. This delivers an offline-capable PWA shell without
 *     risking the inventory / pricing / order flows.
 */

import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;
const ASSETS_CACHE = `gyeon-assets-${version}`;
const PRECACHE: string[] = [...build, ...files];

sw.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(ASSETS_CACHE);
      await cache.addAll(PRECACHE);
      // Activate immediately on first install. On updates the user must close
      // all storefront tabs to pick up the new SW (standard behaviour).
      await sw.skipWaiting();
    })()
  );
});

sw.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.filter((k) => k !== ASSETS_CACHE).map((k) => caches.delete(k)));
      await sw.clients.claim();
    })()
  );
});

sw.addEventListener('fetch', (event) => {
  const req = event.request;

  // Only intercept GET — never cache POST / PUT / DELETE
  if (req.method !== 'GET') return;

  const url = new URL(req.url);

  // Skip cross-origin (analytics scripts, Stripe, etc.)
  if (url.origin !== sw.location.origin) return;

  // Skip dynamic + auth-sensitive paths so they always hit the network.
  if (
    url.pathname.startsWith('/api/') ||
    url.pathname.startsWith('/admin') ||
    url.pathname.startsWith('/uploads/') ||
    url.pathname.startsWith('/account')
  ) {
    return;
  }

  // Cache-first for build outputs and static files; everything else passes
  // through (the network handler will SSR the page fresh).
  const isPrecached = PRECACHE.includes(url.pathname);
  if (!isPrecached) return;

  event.respondWith(
    (async () => {
      const cache = await caches.open(ASSETS_CACHE);
      const cached = await cache.match(req);
      if (cached) return cached;
      const res = await fetch(req);
      // Only cache successful, fully-read responses
      if (res.ok && res.status === 200 && res.type === 'basic') {
        cache.put(req, res.clone()).catch(() => undefined);
      }
      return res;
    })()
  );
});
