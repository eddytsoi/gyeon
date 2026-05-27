import { addToCart, ApiError, getOrCreateCart, removeCartItem, updateCartItem } from '$lib/api';
import type { Cart } from '$lib/types';
import * as m from '$lib/paraglide/messages';

function createCartStore() {
  let cart = $state<Cart | null>(null);
  let loading = $state(false);
  // Surfaced to the storefront layout via a $effect → toast. Null when no
  // error to display. Backend can reject cart-add with 403 (ErrCannotPurchase
  // from the role-purchase gate); without this, the rejection bubbles past
  // the calling component's try/finally and the UI looks like the button is
  // broken. See plan: expressive-mapping-ember.
  let error = $state<string | null>(null);

  const itemCount = $derived(cart?.items?.reduce((sum, i) => sum + i.quantity, 0) ?? 0);
  const subtotal = $derived(
    cart?.items?.reduce((sum, i) => sum + i.price * i.quantity, 0) ?? 0
  );

  function getSessionToken(): string {
    let token = localStorage.getItem('gyeon_session');
    if (!token) {
      token = crypto.randomUUID();
      localStorage.setItem('gyeon_session', token);
    }
    return token;
  }

  function setErrorFromException(e: unknown) {
    if (e instanceof ApiError && e.status === 403 && e.serverMessage) {
      // The backend's 403 message is already user-readable
      // ("this product is not available for your account"); but it's English-only.
      // Localise via the existing role-cannot-purchase message — role is
      // unknown at this layer, so use a generic fallback when it's not the
      // FBT-style role gate (e.g. anonymous + restricted category).
      error = e.serverMessage;
    } else {
      error = m.cart_add_failed();
    }
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
    } catch (e) {
      setErrorFromException(e);
      throw e;
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

  function clearError() {
    error = null;
  }

  return {
    get cart() { return cart; },
    get loading() { return loading; },
    get itemCount() { return itemCount; },
    get subtotal() { return subtotal; },
    get error() { return error; },
    init,
    add,
    update,
    remove,
    clearError
  };
}

export const cartStore = createCartStore();
