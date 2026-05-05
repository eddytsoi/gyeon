/**
 * Storefront analytics tracker (P3 #26). Lazily injects gtag.js and Meta Pixel
 * at most once per page load, only when the corresponding ID is configured in
 * site_settings. All event helpers (trackViewItem, trackAddToCart,
 * trackPurchase) silently no-op if the tracker isn't active — callers don't
 * need to gate their calls.
 *
 * Why gate via runtime settings instead of build-time env vars: the storefront
 * is a single-tenant deploy where the operator can switch GA properties or
 * disable tracking without re-deploying.
 */

import { browser } from '$app/environment';

declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
    fbq?: ((...args: unknown[]) => void) & { callMethod?: (...args: unknown[]) => void; queue?: unknown[]; loaded?: boolean; version?: string; push?: (...args: unknown[]) => void };
    _fbq?: unknown;
  }
}

let initialized = false;
let ga4Id: string | null = null;
let metaPixelId: string | null = null;

/**
 * Call once on first storefront paint with the public_settings array. Loads
 * gtag.js / fbevents.js scripts and primes their global queues. Subsequent
 * calls are ignored.
 */
export function initTracker(settings: Array<{ key: string; value: string }>): void {
  if (!browser || initialized) return;
  initialized = true;

  ga4Id = settings.find((s) => s.key === 'ga4_measurement_id')?.value?.trim() || null;
  metaPixelId = settings.find((s) => s.key === 'meta_pixel_id')?.value?.trim() || null;

  if (ga4Id) installGA4(ga4Id);
  if (metaPixelId) installMetaPixel(metaPixelId);
}

function installGA4(id: string): void {
  // gtag stub queues calls until gtag.js loads. Per Google docs:
  // https://developers.google.com/analytics/devguides/collection/ga4
  window.dataLayer = window.dataLayer || [];
  window.gtag = function gtag() {
    // eslint-disable-next-line prefer-rest-params
    window.dataLayer!.push(arguments);
  };
  window.gtag('js', new Date());
  window.gtag('config', id, { send_page_view: true });

  const s = document.createElement('script');
  s.async = true;
  s.src = `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(id)}`;
  document.head.appendChild(s);
}

function installMetaPixel(id: string): void {
  // Standard Meta Pixel snippet, condensed. The closure pattern primes a
  // queue so events fired before fbevents.js loads aren't dropped.
  /* eslint-disable */
  (function (f: any, b: any, e: any, v: any) {
    if (f.fbq) return;
    const n: any = (f.fbq = function () { n.callMethod ? n.callMethod.apply(n, arguments) : n.queue.push(arguments); });
    if (!f._fbq) f._fbq = n;
    n.push = n; n.loaded = true; n.version = '2.0'; n.queue = [];
    const t = b.createElement(e); t.async = true; t.src = v;
    const s = b.getElementsByTagName(e)[0]; s.parentNode.insertBefore(t, s);
  })(window, document, 'script', 'https://connect.facebook.net/en_US/fbevents.js');
  /* eslint-enable */
  window.fbq!('init', id);
  window.fbq!('track', 'PageView');
}

// ── Event helpers ──────────────────────────────────────────────────────────

interface ItemBase {
  id: string;
  name: string;
  price: number;
  quantity?: number;
  category?: string;
}

export function trackViewItem(item: ItemBase, currency = 'HKD'): void {
  if (!browser) return;
  if (window.gtag) {
    window.gtag('event', 'view_item', {
      currency,
      value: item.price,
      items: [{ item_id: item.id, item_name: item.name, price: item.price }]
    });
  }
  if (window.fbq) {
    window.fbq('track', 'ViewContent', {
      content_ids: [item.id],
      content_name: item.name,
      content_type: 'product',
      value: item.price,
      currency
    });
  }
}

export function trackAddToCart(item: ItemBase, currency = 'HKD'): void {
  if (!browser) return;
  const qty = item.quantity ?? 1;
  if (window.gtag) {
    window.gtag('event', 'add_to_cart', {
      currency,
      value: item.price * qty,
      items: [{ item_id: item.id, item_name: item.name, price: item.price, quantity: qty }]
    });
  }
  if (window.fbq) {
    window.fbq('track', 'AddToCart', {
      content_ids: [item.id],
      content_name: item.name,
      content_type: 'product',
      value: item.price * qty,
      currency
    });
  }
}

export function trackPurchase(orderID: string, total: number, items: ItemBase[], currency = 'HKD'): void {
  if (!browser) return;
  if (window.gtag) {
    window.gtag('event', 'purchase', {
      transaction_id: orderID,
      value: total,
      currency,
      items: items.map((i) => ({
        item_id: i.id,
        item_name: i.name,
        price: i.price,
        quantity: i.quantity ?? 1
      }))
    });
  }
  if (window.fbq) {
    window.fbq('track', 'Purchase', {
      content_ids: items.map((i) => i.id),
      contents: items.map((i) => ({ id: i.id, quantity: i.quantity ?? 1 })),
      content_type: 'product',
      value: total,
      currency
    });
  }
}

export function trackBeginCheckout(total: number, items: ItemBase[], currency = 'HKD'): void {
  if (!browser) return;
  if (window.gtag) {
    window.gtag('event', 'begin_checkout', {
      currency,
      value: total,
      items: items.map((i) => ({ item_id: i.id, item_name: i.name, price: i.price, quantity: i.quantity ?? 1 }))
    });
  }
  if (window.fbq) {
    window.fbq('track', 'InitiateCheckout', {
      content_ids: items.map((i) => i.id),
      value: total,
      currency
    });
  }
}
