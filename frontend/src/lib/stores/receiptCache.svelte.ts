// Tracks which orders have their PDF receipt cached on the backend so the
// admin order page can light up a ⚡ icon next to the download button as
// soon as the queue worker finishes (no page reload needed).
//
// Two write paths feed this:
//   1. Admin SSE `receipt_cache_ready` event (set in (admin)/+layout.svelte).
//   2. Manual mark from a /receipt-cache-status fetch on mount (or after the
//      admin clicks "regenerate" so the icon disappears immediately).
//
// The store is a plain object keyed by order_id. We don't track per-locale
// in the UI — the backend may have multiple locales cached, but the icon
// only cares whether the most-recently-warmed one exists.

function createReceiptCacheStore() {
  const ready = $state<Record<string, boolean>>({});

  return {
    isReady(orderId: string): boolean {
      return ready[orderId] === true;
    },
    set(orderId: string, value: boolean) {
      if (value) {
        ready[orderId] = true;
      } else {
        delete ready[orderId];
      }
    }
  };
}

export const receiptCache = createReceiptCacheStore();
