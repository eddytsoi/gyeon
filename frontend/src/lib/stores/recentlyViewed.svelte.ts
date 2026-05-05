// Recently-viewed product ids, oldest first → newest last. Cap at 8 entries.
// Pure browser-side; never synced to the server (privacy + simplicity).

import { browser } from '$app/environment';

const STORAGE_KEY = 'gyeon.recentlyViewed.v1';
const MAX = 8;

class RecentlyViewedStore {
  ids = $state<string[]>([]);

  init(): void {
    if (!browser) return;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : [];
      this.ids = Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === 'string') : [];
    } catch {
      this.ids = [];
    }
  }

  /** Push (or move to front) a product id, evicting oldest if over MAX. */
  push(productID: string): void {
    if (!browser) return;
    const filtered = this.ids.filter((id) => id !== productID);
    this.ids = [productID, ...filtered].slice(0, MAX);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.ids));
    } catch {
      // ignore quota / privacy errors
    }
  }

  /** Returns ids excluding the current product (so you don't recommend itself). */
  others(currentID: string): string[] {
    return this.ids.filter((id) => id !== currentID);
  }
}

export const recentlyViewedStore = new RecentlyViewedStore();
