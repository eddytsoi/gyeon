<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { decodeSharedCart } from '$lib/cartShare';
  import * as m from '$lib/paraglide/messages';

  // Recipient landing for a shared-cart link. Decodes the ?c= payload, merges
  // each line into the visitor's own session cart, then redirects to /cart.
  onMount(async () => {
    const code = new URLSearchParams(window.location.search).get('c') ?? '';
    const items = decodeSharedCart(code);

    // The (storefront) layout's cartStore.init() may not have run yet — a
    // child page's onMount fires before the parent layout's — so ensure we
    // have a session cart before adding.
    if (!cartStore.cart) {
      await cartStore.init();
    }

    for (const it of items) {
      try {
        await cartStore.add(it.variantId, it.quantity);
      } catch {
        // Out-of-stock / role-restricted lines are skipped; the backend's 403
        // still surfaces via the storefront layout's toast.
      }
    }

    // replaceState so Back doesn't return here and re-add the items.
    await goto('/cart', { replaceState: true });
  });
</script>

<svelte:head>
  <title>{m.cart_shared_loading()}</title>
</svelte:head>

<div class="max-w-3xl mx-auto px-4 sm:px-6 py-20 text-center">
  <p class="text-ink-500">{m.cart_shared_loading()}</p>
</div>
