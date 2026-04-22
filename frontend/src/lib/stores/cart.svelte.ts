import { addToCart, getOrCreateCart, removeCartItem, updateCartItem } from '$lib/api';
import type { Cart } from '$lib/types';

function createCartStore() {
  let cart = $state<Cart | null>(null);
  let loading = $state(false);

  const itemCount = $derived(cart?.items.reduce((sum, i) => sum + i.quantity, 0) ?? 0);

  function getSessionToken(): string {
    let token = localStorage.getItem('gyeon_session');
    if (!token) {
      token = crypto.randomUUID();
      localStorage.setItem('gyeon_session', token);
    }
    return token;
  }

  async function init() {
    if (typeof window === 'undefined') return;
    loading = true;
    try {
      cart = await getOrCreateCart(getSessionToken());
    } finally {
      loading = false;
    }
  }

  async function add(variantID: string, quantity = 1) {
    if (!cart) return;
    loading = true;
    try {
      await addToCart(cart.id, variantID, quantity);
      cart = await getOrCreateCart(getSessionToken());
    } finally {
      loading = false;
    }
  }

  async function update(itemID: string, quantity: number) {
    if (!cart) return;
    loading = true;
    try {
      await updateCartItem(cart.id, itemID, quantity);
      cart = await getOrCreateCart(getSessionToken());
    } finally {
      loading = false;
    }
  }

  async function remove(itemID: string) {
    if (!cart) return;
    loading = true;
    try {
      await removeCartItem(cart.id, itemID);
      cart = await getOrCreateCart(getSessionToken());
    } finally {
      loading = false;
    }
  }

  return {
    get cart() { return cart; },
    get loading() { return loading; },
    get itemCount() { return itemCount; },
    init,
    add,
    update,
    remove
  };
}

export const cartStore = createCartStore();
