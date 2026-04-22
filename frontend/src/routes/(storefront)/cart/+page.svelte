<script lang="ts">
  import { cartStore } from '$lib/stores/cart.svelte';
  import { checkout } from '$lib/api';
  import { goto } from '$app/navigation';

  let placing = $state(false);
  let error = $state('');

  const subtotal = $derived(
    cartStore.cart?.items.reduce((sum) => sum, 0) ?? 0
  );

  async function handleCheckout() {
    if (!cartStore.cart || cartStore.cart.items.length === 0) return;
    placing = true;
    error = '';
    try {
      const order = await checkout(cartStore.cart.id);
      await cartStore.init();
      goto(`/orders/${order.id}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Checkout failed. Please try again.';
    } finally {
      placing = false;
    }
  }
</script>

<svelte:head>
  <title>Cart — Gyeon</title>
</svelte:head>

<div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-8">Your Cart</h1>

  {#if cartStore.loading && !cartStore.cart}
    <div class="text-center py-20 text-gray-400">Loading…</div>

  {:else if !cartStore.cart || cartStore.cart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-gray-400">Your cart is empty.</p>
      <a href="/products"
         class="mt-4 inline-block bg-gray-900 text-white font-medium px-8 py-3
                rounded-full hover:bg-gray-700 transition-colors">
        Continue Shopping
      </a>
    </div>

  {:else}
    <div class="flex flex-col lg:flex-row gap-10">

      <!-- Items -->
      <div class="flex-1 flex flex-col gap-4">
        {#each cartStore.cart.items as item}
          <div class="flex items-center gap-4 bg-white rounded-2xl p-4 border border-gray-100">
            <div class="w-16 h-16 rounded-lg bg-gray-50 flex-shrink-0"></div>

            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-gray-900 truncate">{item.variant_id}</p>
              <p class="text-xs text-gray-400 mt-0.5">SKU: {item.variant_id.slice(0, 8)}…</p>
            </div>

            <!-- Qty controls -->
            <div class="flex items-center border border-gray-200 rounded-lg overflow-hidden">
              <button
                onclick={() => cartStore.update(item.id, item.quantity - 1)}
                class="w-8 h-8 flex items-center justify-center text-gray-500 hover:bg-gray-50">−</button>
              <span class="w-8 text-center text-sm">{item.quantity}</span>
              <button
                onclick={() => cartStore.update(item.id, item.quantity + 1)}
                class="w-8 h-8 flex items-center justify-center text-gray-500 hover:bg-gray-50">+</button>
            </div>

            <button
              onclick={() => cartStore.remove(item.id)}
              class="p-2 text-gray-300 hover:text-red-400 transition-colors"
              aria-label="Remove">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
                   viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        {/each}
      </div>

      <!-- Summary -->
      <div class="lg:w-72 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 sticky top-24">
          <h2 class="font-semibold text-gray-900">Order Summary</h2>

          <div class="flex justify-between text-sm text-gray-600">
            <span>Items ({cartStore.itemCount})</span>
            <span>—</span>
          </div>
          <div class="flex justify-between text-sm text-gray-600">
            <span>Shipping</span>
            <span class="text-green-600">Free</span>
          </div>
          <div class="border-t border-gray-100 pt-3 flex justify-between font-semibold text-gray-900">
            <span>Total</span>
            <span>—</span>
          </div>

          {#if error}
            <p class="text-xs text-red-500">{error}</p>
          {/if}

          <button
            onclick={handleCheckout}
            disabled={placing}
            class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                   hover:bg-gray-700 transition-colors disabled:opacity-50">
            {placing ? 'Placing order…' : 'Checkout'}
          </button>

          <a href="/products" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            ← Continue Shopping
          </a>
        </div>
      </div>
    </div>
  {/if}
</div>
