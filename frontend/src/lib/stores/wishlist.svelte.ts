// Unified wishlist store. Talks to /api/wishlist/* SvelteKit proxy endpoints
// that translate the httpOnly customer_token cookie into a Bearer token for
// the Go backend, so the client never holds the JWT directly.
//
// Guest path uses localStorage; on login the layout calls merge() to push the
// guest list to the server, then clears localStorage.

import { browser } from '$app/environment';

const STORAGE_KEY = 'gyeon.wishlist.v1';

export interface WishlistApiItem {
  id: string;
  product_id: string;
  product_slug?: string;
  product_name?: string;
  product_image_url?: string;
  created_at: string;
}

class WishlistStore {
  ids = $state<string[]>([]);
  items = $state<WishlistApiItem[]>([]);
  loaded = $state(false);
  /** Whether the user is authenticated. The proxy decides via cookie either
   *  way; we just use this flag to choose between localStorage and API.    */
  private authenticated = false;

  has(productID: string): boolean {
    return this.ids.includes(productID);
  }

  /** Called from (storefront)/+layout.svelte when we know auth state. */
  async init(authenticated: boolean): Promise<void> {
    if (!browser) return;
    this.authenticated = authenticated;
    if (authenticated) {
      // If a guest list exists from before login, merge it into the server.
      const guest = this.readLocal();
      if (guest.length) {
        await this.mergeGuest(guest);
        this.writeLocal([]);
      } else {
        await this.refresh();
      }
    } else {
      const ids = this.readLocal();
      this.ids = ids;
      this.items = ids.map((pid) => ({ id: pid, product_id: pid, created_at: '' }));
      this.loaded = true;
    }
  }

  async toggle(productID: string): Promise<boolean> {
    if (this.has(productID)) {
      await this.remove(productID);
      return false;
    }
    await this.add(productID);
    return true;
  }

  async add(productID: string): Promise<void> {
    if (this.has(productID)) return;
    if (this.authenticated) {
      const res = await fetch('/api/wishlist', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ product_id: productID })
      });
      if (res.ok) await this.refresh();
    } else {
      this.ids = [productID, ...this.ids];
      this.items = [{ id: productID, product_id: productID, created_at: '' }, ...this.items];
      this.writeLocal(this.ids);
    }
  }

  async remove(productID: string): Promise<void> {
    if (this.authenticated) {
      const res = await fetch(`/api/wishlist/${encodeURIComponent(productID)}`, { method: 'DELETE' });
      if (res.ok) await this.refresh();
    } else {
      this.ids = this.ids.filter((id) => id !== productID);
      this.items = this.items.filter((it) => it.product_id !== productID);
      this.writeLocal(this.ids);
    }
  }

  async refresh(): Promise<void> {
    if (!this.authenticated) return;
    try {
      const res = await fetch('/api/wishlist');
      if (!res.ok) return;
      const data = (await res.json()) as WishlistApiItem[];
      this.items = data;
      this.ids = data.map((it) => it.product_id);
    } finally {
      this.loaded = true;
    }
  }

  private async mergeGuest(productIDs: string[]): Promise<void> {
    try {
      const res = await fetch('/api/wishlist/merge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ product_ids: productIDs })
      });
      if (res.ok) {
        const data = (await res.json()) as WishlistApiItem[];
        this.items = data;
        this.ids = data.map((it) => it.product_id);
      }
    } finally {
      this.loaded = true;
    }
  }

  private readLocal(): string[] {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === 'string') : [];
    } catch {
      return [];
    }
  }

  private writeLocal(ids: string[]): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(ids));
    } catch {
      // ignore quota / privacy mode errors
    }
  }
}

export const wishlistStore = new WishlistStore();
