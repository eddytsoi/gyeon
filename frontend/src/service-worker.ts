/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true" />
/// <reference lib="esnext" />
/// <reference lib="webworker" />

/*
 * P3 #25 — service worker for offline app shell.
 *
 * SvelteKit injects the build manifest at compile time via $service-worker.
 * `build` = hashed app assets (immutable, safe to long-cache).
 * `files` = static/ contents (includes /offline.html).
 * `prerendered` = unused here (no prerendered routes).
 *
 * Strategy:
 *   - Install: pre-cache the build + static manifest under ASSETS_CACHE.
 *   - Fetch: cache-first for those assets; pass through everything else
 *     (storefront pages, /api/*, /uploads/*) so the SW never serves stale
 *     dynamic content. This delivers an offline-capable PWA shell without
 *     risking the inventory / pricing / order flows.
 *   - Navigation fallback: if a top-level page request fails (offline),
 *     serve /offline.html so installed PWA users see a branded screen
 *     instead of a black/error page.
 */

import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;
const ASSETS_CACHE = `gyeon-assets-${version}`;
const OFFLINE_URL = '/offline.html';
const PRECACHE: string[] = [...build, ...files];
if (!PRECACHE.includes(OFFLINE_URL)) PRECACHE.push(OFFLINE_URL);

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

  // Top-level navigation: try the network, fall back to /offline.html
  // when the device is offline so installed PWA users see a branded
  // screen instead of a black/error page.
  if (req.mode === 'navigate') {
    event.respondWith(
      (async () => {
        try {
          return await fetch(req);
        } catch {
          const cache = await caches.open(ASSETS_CACHE);
          const offline = await cache.match(OFFLINE_URL);
          if (offline) return offline;
          return new Response('Offline', {
            status: 503,
            headers: { 'Content-Type': 'text/plain; charset=utf-8' }
          });
        }
      })()
    );
    return;
  }

  // SvelteKit SPA navigation fetches `<route>/__data.json` instead of
  // doing a full document load — so `req.mode` is not 'navigate' and the
  // handler above never fires. Without intercepting these, an offline
  // in-app link click falls through to the network, the load() throws,
  // and SvelteKit renders its default 500 error page. Returning 503 here
  // lets the root +error.svelte detect offline and redirect to
  // /offline.html for a consistent branded experience.
  if (url.pathname.endsWith('/__data.json')) {
    event.respondWith(
      (async () => {
        try {
          return await fetch(req);
        } catch {
          return new Response('{"type":"error","error":{"message":"Offline"}}', {
            status: 503,
            headers: { 'Content-Type': 'application/json; charset=utf-8' }
          });
        }
      })()
    );
    return;
  }

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
